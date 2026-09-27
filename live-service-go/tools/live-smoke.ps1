# live-service-go 端到端冒烟（需 live-service 已在 9002 运行）
# 用法：powershell -NoProfile -ExecutionPolicy Bypass -File tools/live-smoke.ps1
#
# 契约要点（别按直觉猜，这几条是读源码/实测确认过的）：
#  1. Start 每次都生成**新的 streamKey/sessionId**；对已处于 STARTING/LIVING 的房间
#     是幂等的（原样返回当前密钥，不换）。所以拿 on_publish 冒烟必须用 **Start 返回的**
#     streamKey，用创建时那个会得到 70002「推流密钥无效」。
#  2. 主动关播后状态是 **ENDED**（不是 IDLE）；IDLE 只表示「创建后从未开播」。
#     ENDED 仍可再次 Start（CanStart = IDLE|ENDED|FAILED）。
#  3. `GET /:id/records` 在 authed 组内，不带 X-User-Id 会 40101。

$ErrorActionPreference = "Stop"
$base = "http://localhost:9002"
$uid = "u-7001"

$script:pass = 0
$script:fail = 0
function Check([string]$name, [bool]$ok, [string]$detail) {
    if ($ok) { $script:pass++; Write-Host ("  [PASS] " + $name) -ForegroundColor Green }
    else { $script:fail++; Write-Host ("  [FAIL] " + $name + "  -> " + $detail) -ForegroundColor Red }
}

# 绝不能把 JSON 直接当 -d 参数传给 curl.exe —— PowerShell 会剥掉内层引号、
# 拆散花括号，curl 收到残缺参数（报 `option -d: requires parameter`）。
# 统一写 **UTF-8 无 BOM** 临时文件，再用 --data-binary "@文件"。
function New-BodyFile([string]$json) {
    $f = Join-Path $env:TEMP ("live-smoke-" + [guid]::NewGuid().ToString("N") + ".json")
    [System.IO.File]::WriteAllText($f, $json, (New-Object System.Text.UTF8Encoding($false)))
    return $f
}

function Post([string]$path, [string]$body, [string]$user) {
    $curlArgs = @("-s", "-X", "POST", "$base$path")
    if ($user) { $curlArgs += @("-H", "X-User-Id: $user") }
    $f = $null
    if ($body) {
        $f = New-BodyFile $body
        $curlArgs += @("-H", "Content-Type: application/json", "--data-binary", "@$f")
    }
    try { return (& curl.exe @curlArgs) } finally { if ($f -and (Test-Path $f)) { Remove-Item $f -Force } }
}

function Get-J([string]$path, [string]$user) {
    $curlArgs = @("-s", "$base$path")
    if ($user) { $curlArgs += @("-H", "X-User-Id: $user") }
    return (& curl.exe @curlArgs)
}

Write-Host "`n[1] 未登录创建直播间应 401" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms" '{"title":"x"}'
Check "未登录被拒" ($r -like '*"code":40101*') $r

Write-Host "`n[2] 创建直播间" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms" '{"title":"e2e live room","notice":"smoke"}' $uid
$room = $r | ConvertFrom-Json
Check "创建成功" ($room.code -eq 0) $r
Check "初始状态 IDLE（从未开播）" ($room.data.status -eq 'IDLE') $room.data.status
Check "返回 streamKey" ([bool]$room.data.streamKey) $room.data.streamKey
Check "返回推流地址" ($room.data.pushUrl -like 'rtmp://*') $room.data.pushUrl
$id = $room.data.roomId
Write-Host ("  roomId=$id 创建时 streamKey=$($room.data.streamKey)")

Write-Host "`n[3] 开播：应转入 STARTING 并下发新的 streamKey/sessionId" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms/$id/start" "" $uid
$start = ($r | ConvertFrom-Json).data
Check "开播成功" ($start.status -eq 'STARTING') $r
Check "生成 sessionId" ([bool]$start.sessionId) $start.sessionId
Check "streamKey 与创建时不同（每次开播换密钥）" ($start.streamKey -ne $room.data.streamKey) "$($start.streamKey) vs $($room.data.streamKey)"
Check "返回 HLS 地址" ($start.playUrls.hls -like '*m3u8') $start.playUrls.hls
$key = $start.streamKey
$session = $start.sessionId
Write-Host ("  开播 streamKey=$key sessionId=$session")

Write-Host "`n[4] 重复开播应幂等（状态不变、密钥不换）" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms/$id/start" "" $uid
$again = ($r | ConvertFrom-Json).data
Check "重复开播返回原状态" ($again.status -eq 'STARTING') $again.status
Check "streamKey 未被替换" ($again.streamKey -eq $key) "$($again.streamKey) vs $key"
Check "sessionId 未被替换" ($again.sessionId -eq $session) "$($again.sessionId) vs $session"

Write-Host "`n[5] 用**旧的**（创建时）streamKey 推流应被拒 70002" -ForegroundColor Cyan
$r = curl.exe -s -X POST "$base/api/v1/live/dev/publish/$($room.data.streamKey)"
Check "旧密钥被拒 70002" ($r -like '*"code":70002*') $r

Write-Host "`n[6] 模拟 SRS on_publish（带 session）→ LIVING" -ForegroundColor Cyan
# 必须写成 ${key} / ${session}：PowerShell 里 `?` **是变量名的合法字符**，
# "$key?session=" 会被当成变量 `key?session`（未定义）再拼上 "=$session"，
# 实际请求会变成 /dev/publish/=80d898…（真踩过，日志里一眼可辨）。
$url = "$base/api/v1/live/dev/publish/${key}?session=${session}"
$r = curl.exe -s -X POST $url
Check "on_publish 成功" ($r -like '*"code":0*') "$r  url=$url"
$detail = ((Get-J "/api/v1/live/rooms/$id") | ConvertFrom-Json).data
Check "状态已变 LIVING（只有 on_publish 能推进）" ($detail.status -eq 'LIVING') $detail.status
Check "startedAt 已写入" ([bool]$detail.startedAt) $detail.startedAt

Write-Host "`n[7] LIVING 时播放地址应给出多种协议" -ForegroundColor Cyan
$play = ((Get-J "/api/v1/live/rooms/$id/play") | ConvertFrom-Json).data
Check "rtmp 地址" ($play.playUrls.rtmp -like 'rtmp://*') $play.playUrls.rtmp
Check "httpFlv 地址" ($play.playUrls.httpFlv -like 'http://*') $play.playUrls.httpFlv
Check "hls 地址" ($play.playUrls.hls -like '*m3u8') $play.playUrls.hls

Write-Host "`n[8] 观众心跳与在线人数" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms/$id/heartbeat" ("{""sessionId"":""$session""}") "u-8001"
Check "观众1 心跳成功" ($r -like '*"code":0*') $r
$r = Post "/api/v1/live/rooms/$id/heartbeat" ("{""sessionId"":""$session""}") "u-8002"
Check "观众2 心跳成功" ($r -like '*"code":0*') $r
$online = ((Get-J "/api/v1/live/rooms/$id/online") | ConvertFrom-Json).data
Check "在线人数为 2" ($online.onlineCount -eq 2) $online.onlineCount
Check "人次也计到 2" ($online.viewerTotal -eq 2) $online.viewerTotal

Write-Host "`n[9] 列表接口" -ForegroundColor Cyan
$list = ((Get-J "/api/v1/live/rooms?page=1&pageSize=5") | ConvertFrom-Json).data
$hit = @($list.items | Where-Object { $_.roomId -eq $id })
Check "列表包含刚建房间" ($hit.Count -ge 1) "count=$($hit.Count)"

Write-Host "`n[10] 非房主不能关播" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms/$id/stop" "" "u-9999"
Check "非房主被拒 403" ($r -like '*"code":40301*') $r

Write-Host "`n[11] 下播：状态转 ENDED（不是 IDLE），并落一条场次记录" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms/$id/stop" "" $uid
$stop = ($r | ConvertFrom-Json).data
Check "下播成功" ($stop.status -eq 'ENDED') $stop.status
Check "返回时长" ($stop.durationSec -ge 0) $stop.durationSec
Check "峰值人数已统计" ($stop.peakOnline -ge 2) $stop.peakOnline
$recs = ((Get-J "/api/v1/live/rooms/$id/records" $uid) | ConvertFrom-Json).data
Check "有 1 条场次记录" ($recs.total -ge 1) $recs.total
Check "记录里 peakOnline >= 2" (@($recs.items)[0].peakOnline -ge 2) (@($recs.items)[0].peakOnline)
Check "记录里 viewerTotal >= 2" (@($recs.items)[0].viewerTotal -ge 2) (@($recs.items)[0].viewerTotal)

Write-Host "`n[12] 下播后心跳应被拒（不再直播中）" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms/$id/heartbeat" ("{""sessionId"":""$session""}") "u-8001"
Check "下播后心跳被拒 70001" ($r -like '*"code":70001*') $r

Write-Host "`n[13] ENDED 的房间可以再次开播" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms/$id/start" "" $uid
$restart = ($r | ConvertFrom-Json).data
Check "再次开播成功" ($restart.status -eq 'STARTING') $r
Check "又换了新 streamKey" ($restart.streamKey -ne $key) "$($restart.streamKey) vs $key"

Write-Host "`n[14] 清理：先关播再删除房间" -ForegroundColor Cyan
Post "/api/v1/live/rooms/$id/stop" "" $uid | Out-Null
$r = curl.exe -s -X DELETE "$base/api/v1/live/rooms/$id" -H "X-User-Id: $uid"
Check "删除成功" ($r -like '*"code":0*') $r
$r = Get-J "/api/v1/live/rooms/$id"
Check "删除后详情 404（70005 直播间不存在）" ($r -like '*"code":70005*') $r

Write-Host "`n========== 结果：通过 $script:pass 项，失败 $script:fail 项 ==========" -ForegroundColor Cyan
if ($script:fail -gt 0) { exit 1 }
Write-Host "全部通过" -ForegroundColor Green
