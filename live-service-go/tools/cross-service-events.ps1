# 跨服务事件链路验证（live-service 9002 + danmaku-service 9003 都需已启动）
# 用法：powershell -NoProfile -ExecutionPolicy Bypass -File tools/cross-service-events.ps1
#
# 验证的东西（这是「三个服务共用一套 MySQL/Redis + RabbitMQ」最容易漏掉的一环）：
#   live-service 发布 live.started → RabbitMQ 投递到 danmaku-service **自己的**队列
#   → danmaku 的 LiveBridge 收到 → 向该房间所有 WS 连接广播 system 帧
#   下播时同理广播 LIVE_ENDED。
#
# 关键点：PowerShell 里 `?` 是变量名的合法字符，拼 URL 必须写 ${var}，
# 否则 "$key?session=" 会被当成变量 `key?session`（不要问我怎么知道的）。

$ErrorActionPreference = "Stop"
$liveBase = "http://localhost:9002"
$uid = "u-7100"

$script:pass = 0
$script:fail = 0
function Check([string]$name, [bool]$ok, [string]$detail) {
    if ($ok) { $script:pass++; Write-Host ("  [PASS] " + $name) -ForegroundColor Green }
    else { $script:fail++; Write-Host ("  [FAIL] " + $name + "  -> " + $detail) -ForegroundColor Red }
}
function New-BodyFile([string]$json) {
    $f = Join-Path $env:TEMP ("xsvc-" + [guid]::NewGuid().ToString("N") + ".json")
    [System.IO.File]::WriteAllText($f, $json, (New-Object System.Text.UTF8Encoding($false)))
    return $f
}
function Post([string]$path, [string]$body, [string]$user) {
    $a = @("-s", "-X", "POST", "$liveBase$path")
    if ($user) { $a += @("-H", "X-User-Id: $user") }
    $f = $null
    if ($body) { $f = New-BodyFile $body; $a += @("-H", "Content-Type: application/json", "--data-binary", "@$f") }
    try { return (& curl.exe @a) } finally { if ($f -and (Test-Path $f)) { Remove-Item $f -Force } }
}
function Receive-Frame($ws, [string]$label, [int]$timeoutMs = 8000) {
    $buf = New-Object byte[] 16384
    $seg = New-Object System.ArraySegment[byte] -ArgumentList @(, $buf)
    $task = $ws.ReceiveAsync($seg, [Threading.CancellationToken]::None)
    if (-not $task.Wait($timeoutMs)) { Write-Host ("  $label <= (超时)") -ForegroundColor Yellow; return $null }
    if ($task.Result.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
        return "CLOSE:$([int]$task.Result.CloseStatus)"
    }
    $text = [Text.Encoding]::UTF8.GetString($buf, 0, $task.Result.Count)
    Write-Host "  $label <= $text"
    return $text
}

Write-Host "`n[1] live-service 建房间并开播" -ForegroundColor Cyan
$room = ((Post "/api/v1/live/rooms" '{"title":"cross svc room"}' $uid) | ConvertFrom-Json).data
$id = $room.roomId
$start = ((Post "/api/v1/live/rooms/$id/start" "" $uid) | ConvertFrom-Json).data
$key = $start.streamKey
$session = $start.sessionId
Check "开播进入 STARTING" ($start.status -eq 'STARTING') $start.status
Write-Host ("  roomId=$id streamKey=$key sessionId=$session")

Write-Host "`n[2] 连一个 WS 到 danmaku-service 的同一房间（welcome 里 readOnly 应为 false）" -ForegroundColor Cyan
$ws = New-Object System.Net.WebSockets.ClientWebSocket
$ws.ConnectAsync([Uri]"ws://localhost:9003/ws/danmaku?roomId=${id}&uid=u-7100&nickname=watcher", [Threading.CancellationToken]::None).Wait()
$f = Receive-Frame $ws "welcome"
Check "收到 welcome" ($f -like '*"type":"welcome"*') $f
Check "welcome 里 roomId 一致" ($f -like "*`"roomId`":`"$id`"*") $f

Write-Host "`n[3] 触发 on_publish → live-service 发 live.started → danmaku 应广播 system LIVE_STARTED" -ForegroundColor Cyan
$r = curl.exe -s -X POST "$liveBase/api/v1/live/dev/publish/${key}?session=${session}"
Check "on_publish 成功" ($r -like '*"code":0*') $r
$f = Receive-Frame $ws "ws(system)"
Check "收到 system 帧" ($f -like '*"type":"system"*') $f
Check "事件为 LIVE_STARTED" ($f -like '*LIVE_STARTED*') $f

Write-Host "`n[4] 关播 → live.stopped → danmaku 应广播 system LIVE_ENDED" -ForegroundColor Cyan
$r = Post "/api/v1/live/rooms/$id/stop" "" $uid
Check "关播成功" ($r -like '*"code":0*') $r
$f = Receive-Frame $ws "ws(system)"
Check "收到 system 帧" ($f -like '*"type":"system"*') $f
Check "事件为 LIVE_ENDED" ($f -like '*LIVE_ENDED*') $f

Write-Host "`n[5] 直播已结束：此时该房间发弹幕应被拒（danmaku 已收到 live.stopped）" -ForegroundColor Cyan
$send = New-BodyFile ("{""roomId"":""$id"",""content"":""after ended""}")
$r = curl.exe -s -X POST "http://localhost:9003/api/v1/danmaku/send" -H "X-User-Id: u-7100" -H "Content-Type: application/json" --data-binary "@$send"
Remove-Item $send -Force
Check "直播已结束后发送被拒 80003" ($r -like '*"code":80003*') $r

$ws.Dispose()

Write-Host "`n[6] 清理房间" -ForegroundColor Cyan
$r = curl.exe -s -X DELETE "$liveBase/api/v1/live/rooms/$id" -H "X-User-Id: $uid"
Check "删除成功" ($r -like '*"code":0*') $r

Write-Host "`n========== 结果：通过 $script:pass 项，失败 $script:fail 项 ==========" -ForegroundColor Cyan
if ($script:fail -gt 0) { exit 1 }
Write-Host "全部通过" -ForegroundColor Green
