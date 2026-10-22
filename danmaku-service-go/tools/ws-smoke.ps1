# danmaku-service-go WebSocket 冒烟脚本
# 依赖：仅需 PowerShell 自带的 System.Net.WebSockets.ClientWebSocket，无需任何第三方包。
# 前置：danmaku-service 已在 9003 端口运行（go run ./cmd/server）。
# 用法：powershell -NoProfile -ExecutionPolicy Bypass -File tools/ws-smoke.ps1
#
# 注意：Receive-Frame 一律「同步等到一帧」——绝不做「等超时看有没有帧」的断言。
# 因为 ClientWebSocket 只允许一个未完成的 ReceiveAsync，超时放弃后会留下孤儿任务，
# 把后续要读的那一帧吃掉，导致后面全是假超时（这个坑真踩过，改成顺序断言）。

$ErrorActionPreference = "Stop"
$base = "ws://localhost:9003/ws/danmaku"

$script:pass = 0
$script:fail = 0

function Check([string]$name, [bool]$ok, [string]$detail) {
    if ($ok) {
        $script:pass++
        Write-Host ("  [PASS] " + $name) -ForegroundColor Green
    }
    else {
        $script:fail++
        Write-Host ("  [FAIL] " + $name + "  -> " + $detail) -ForegroundColor Red
    }
}

function New-WsClient([string]$url) {
    $ws = New-Object System.Net.WebSockets.ClientWebSocket
    $ws.ConnectAsync([Uri]$url, [Threading.CancellationToken]::None).Wait()
    return $ws
}

function Receive-Frame($ws, [string]$label, [int]$timeoutMs = 5000) {
    $buf = New-Object byte[] 16384
    $seg = New-Object System.ArraySegment[byte] -ArgumentList @(, $buf)
    $task = $ws.ReceiveAsync($seg, [Threading.CancellationToken]::None)
    if (-not $task.Wait($timeoutMs)) {
        Write-Host ("  $label <= (超时，未收到帧)") -ForegroundColor Yellow
        return $null
    }
    if ($task.Result.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
        $code = [int]$task.Result.CloseStatus
        Write-Host ("  $label <= (关闭帧 code=$code " + $task.Result.CloseStatusDescription + ")") -ForegroundColor Magenta
        return "CLOSE:$code"
    }
    $text = [Text.Encoding]::UTF8.GetString($buf, 0, $task.Result.Count)
    Write-Host "  $label <= $text"
    return $text
}

function Send-Frame($ws, [string]$json) {
    $bytes = [Text.Encoding]::UTF8.GetBytes($json)
    $seg = New-Object System.ArraySegment[byte] -ArgumentList @(, $bytes)
    $ws.SendAsync($seg, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, [Threading.CancellationToken]::None).Wait()
}

Write-Host "`n[1] 握手：三个连接都应收到 welcome" -ForegroundColor Cyan
$alice = New-WsClient "${base}?roomId=5001&uid=u-2001&nickname=alice"
$bob = New-WsClient "${base}?roomId=5001&uid=u-3001&nickname=bob"
$guest = New-WsClient "${base}?videoId=1001"

$f = Receive-Frame $alice "alice"
Check "alice welcome" ($f -like '*"type":"welcome"*') $f
Check "alice 登录用户非只读" ($f -like '*"readOnly":false*') $f

$f = Receive-Frame $bob "bob"
Check "bob welcome onlineCount=2" ($f -like '*"onlineCount":2*') $f

$f = Receive-Frame $guest "guest"
Check "guest(点播) welcome" ($f -like '*"type":"welcome"*') $f
Check "guest 未登录 readOnly=true" ($f -like '*"readOnly":true*') $f
Check "guest 在线人数按视频维度隔离" ($f -like '*"onlineCount":1*') $f

Write-Host "`n[2] 房间广播：alice 发弹幕，同房间 bob 必须立刻收到，alice 自己也收到" -ForegroundColor Cyan
Send-Frame $alice '{"type":"danmaku","data":{"content":"cross connection test","clientMsgId":"ws-001"}}'
$f = Receive-Frame $bob "bob"
Check "bob 收到 alice 的弹幕" (($f -like '*"type":"danmaku"*') -and ($f -like '*cross connection test*')) $f
Check "广播里带上昵称" ($f -like '*"nickname":"alice"*') $f
$f = Receive-Frame $alice "alice"
Check "发送者自己也通过广播收到（不单独回包）" ($f -like '*"clientMsgId":"ws-001"*') $f

Write-Host "`n[3] 房间隔离：guest 在 v:1001，紧跟着发 ping 应立刻收到 pong（若收到 danmaku 即跨房间泄漏）" -ForegroundColor Cyan
Send-Frame $guest '{"type":"ping","data":{}}'
$f = Receive-Frame $guest "guest"
Check "guest 下一帧是 pong（说明没被别的房间的广播污染）" ($f -like '*"type":"pong"*') $f

Write-Host "`n[4] 只读游客发弹幕：应回 error(40101)，且连接不能断" -ForegroundColor Cyan
Send-Frame $guest '{"type":"danmaku","data":{"content":"guest should not send","clientMsgId":"guest-1"}}'
$f = Receive-Frame $guest "guest"
Check "游客发送被拒 40101" ($f -like '*"code":40101*') $f
Send-Frame $guest '{"type":"ping","data":{}}'
$f = Receive-Frame $guest "guest"
Check "被拒后连接仍存活（业务错误不关连接）" ($f -like '*"type":"pong"*') $f

Write-Host "`n[5] 单用户限流：bob 连发 8 条，前 5 条通过、后 3 条 80002" -ForegroundColor Cyan
for ($i = 1; $i -le 8; $i++) {
    Send-Frame $bob "{`"type`":`"danmaku`",`"data`":{`"content`":`"flood $i`",`"clientMsgId`":`"ws-flood-$i`"}}"
    $f = Receive-Frame $bob "bob#$i"
    if ($i -le 5) {
        Check "第 $i 条限流前放行" ($f -like '*"type":"danmaku"*') $f
    }
    else {
        Check "第 $i 条被限流 80002" ($f -like '*"code":80002*') $f
        Check "第 $i 条带 retryAfterMs" ($f -like '*retryAfterMs*') $f
    }
}

Write-Host "`n[6] 连续 3 次非法帧：每次回 error(40001)，第 3 次后服务端以 1003 关闭" -ForegroundColor Cyan
for ($i = 1; $i -le 3; $i++) {
    Send-Frame $bob "not json $i"
    $f = Receive-Frame $bob "bob(畸形帧 #$i)"
    Check "畸形帧 #$i 回 40001" ($f -like '*"code":40001*') $f
}
$f = Receive-Frame $bob "bob(关闭帧)"
Check "第 3 次协议错误后用 1003 关闭（没被收尾的 1000 覆盖）" ($f -eq "CLOSE:1003") $f

Write-Host "`n[7] 单用户连接上限：同一 uid 第 4 条连接应把最旧的踢掉（4001）" -ForegroundColor Cyan
$c1 = New-WsClient "${base}?roomId=5002&uid=u-9001&nickname=k1"
Receive-Frame $c1 "c1" | Out-Null
$c2 = New-WsClient "${base}?roomId=5002&uid=u-9001&nickname=k2"
Receive-Frame $c2 "c2" | Out-Null
$c3 = New-WsClient "${base}?roomId=5002&uid=u-9001&nickname=k3"
Receive-Frame $c3 "c3" | Out-Null
$c4 = New-WsClient "${base}?roomId=5002&uid=u-9001&nickname=k4"
Receive-Frame $c4 "c4" | Out-Null
$f = Receive-Frame $c1 "c1(被踢)"
Check "最旧的连接被 4001 踢掉" ($f -eq "CLOSE:4001") $f

foreach ($c in @($alice, $bob, $guest, $c1, $c2, $c3, $c4)) { $c.Dispose() }

Write-Host "`n========== 结果：通过 $script:pass 项，失败 $script:fail 项 ==========" -ForegroundColor Cyan
if ($script:fail -gt 0) { exit 1 }
Write-Host "全部通过" -ForegroundColor Green
