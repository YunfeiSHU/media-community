# video-service-go 分片上传 / 断点续传端到端冒烟
# 用法：powershell -NoProfile -ExecutionPolicy Bypass -File tools/upload-chunk-smoke.ps1
#
# 为什么单独一个脚本（不并进 video-smoke.ps1）：
#   S3 协议规定「非末片不得小于 5MiB」，所以这里必须真造一个 >10MiB 的文件并切片上传，
#   耗时明显高于其它用例；混进日常回归会让 video-smoke 变慢。
#
# 契约要点（读源码确认过，别按直觉猜）：
#  1. init 的 fileSize 决定切片计划：11MiB + 默认下限 5MiB → partSize=5242880、totalParts=3（5+5+1）。
#  2. 上传分片是**原始二进制**（PUT + Content-Length），不是 multipart 表单。
#  3. 非末片大小必须**正好等于** partSize，否则 60005；末片只能更小。
#  4. 已上传分片的权威数据在 MinIO 侧（ListParts），所以「断点续传」= 再查一次 status。
#  5. 同参数的 init 会**复用**同一 uploadId（幂等），不会把已传分片废掉。
#  6. 错误码：40404=上传会话不存在，60005=分片不合法，60006=分片未齐/总大小不符。

$ErrorActionPreference = "Stop"
$base = "http://localhost:9001"
$uid = "u-7001"

$script:pass = 0
$script:fail = 0
function Check([string]$name, [bool]$ok, [string]$detail) {
    if ($ok) { $script:pass++; Write-Host ("  [PASS] " + $name) -ForegroundColor Green }
    else { $script:fail++; Write-Host ("  [FAIL] " + $name + "  -> " + $detail) -ForegroundColor Red }
}
function New-BodyFile([string]$json) {
    $f = Join-Path $env:TEMP ("upload-smoke-" + [guid]::NewGuid().ToString("N") + ".json")
    [System.IO.File]::WriteAllText($f, $json, (New-Object System.Text.UTF8Encoding($false)))
    return $f
}
function Post([string]$path, [string]$body, [string]$user) {
    $a = @("-s", "-X", "POST", "$base$path")
    if ($user) { $a += @("-H", "X-User-Id: $user") }
    $f = $null
    if ($body) { $f = New-BodyFile $body; $a += @("-H", "Content-Type: application/json", "--data-binary", "@$f") }
    try { return (& curl.exe @a) } finally { if ($f -and (Test-Path $f)) { Remove-Item $f -Force } }
}
function Get-J([string]$path, [string]$user) {
    $a = @("-s", "$base$path")
    if ($user) { $a += @("-H", "X-User-Id: $user") }
    return (& curl.exe @a)
}
function Put-Bin([string]$path, [string]$file, [string]$user) {
    $a = @("-s", "-X", "PUT", "$base$path", "-H", "X-User-Id: $user",
           "-H", "Content-Type: application/octet-stream", "--data-binary", "@$file")
    return (& curl.exe @a)
}
# 把整文件按 [offset, offset+size) 切出来写成独立分片文件。
# 用 [Array]::Copy 而不是 PS 的数组切片（$bytes[0..n]）——后者对 5M 元素要慢一个数量级。
function Split-Part([byte[]]$all, [int]$offset, [int]$size, [string]$dest) {
    $len = [Math]::Min($size, $all.Length - $offset)
    $buf = New-Object byte[] $len
    [Array]::Copy($all, $offset, $buf, 0, $len)
    [System.IO.File]::WriteAllBytes($dest, $buf)
    return $len
}

$mib = 1MB
$fileSize = 11 * $mib          # 11MiB
$partSize = 5 * $mib           # 服务端下限，也是 init 返回的 partSize
$totalParts = 3                # 5 + 5 + 1

Write-Host "`n[1] 服务就绪" -ForegroundColor Cyan
$r = (Get-J "/readyz") | ConvertFrom-Json
Check "readyz UP" ($r.data.status -eq 'UP') ($r | ConvertTo-Json -Compress)

Write-Host "`n[2] 创建草稿 + 准备 11MiB 测试文件" -ForegroundColor Cyan
$src = Join-Path $env:TEMP ("big-" + [guid]::NewGuid().ToString("N") + ".mp4")
$all = New-Object byte[] $fileSize
for ($i = 0; $i -lt $fileSize; $i++) { $all[$i] = [byte]($i % 256) }
[System.IO.File]::WriteAllBytes($src, $all)
Check "测试文件已生成 $fileSize 字节" ((Get-Item $src).Length -eq $fileSize) (Get-Item $src).Length

$partFiles = @()
for ($i = 0; $i -lt $totalParts; $i++) {
    $f = Join-Path $env:TEMP ("part-" + $i + "-" + [guid]::NewGuid().ToString("N") + ".bin")
    $len = Split-Part $all ($i * $partSize) $partSize $f
    $partFiles += $f
    Write-Host ("  part#" + ($i + 1) + " = " + $len + " 字节 -> " + (Split-Path $f -Leaf))
}

$v = (Post "/api/v1/videos" '{"title":"chunked upload e2e"}' $uid) | ConvertFrom-Json
Check "创建草稿成功" ($v.code -eq 0) ($v | ConvertTo-Json -Compress)
$id = $v.data.videoId
Write-Host ("  videoId=$id")

Write-Host "`n[3] init：服务端推导切片计划" -ForegroundColor Cyan
$body = '{"filename":"big.mp4","fileSize":' + $fileSize + '}'
$s = (Post "/api/v1/videos/$id/upload/init" $body $uid) | ConvertFrom-Json
Check "init 成功" ($s.code -eq 0) ($s | ConvertTo-Json -Compress)
Check "partSize = 5MiB" ($s.data.partSize -eq $partSize) $s.data.partSize
Check "totalParts = 3" ($s.data.totalParts -eq $totalParts) $s.data.totalParts
Check "初始已上传 0 片" ($s.data.uploadedCount -eq 0) $s.data.uploadedCount
Check "返回 uploadId" (-not [string]::IsNullOrWhiteSpace($s.data.uploadId)) $s.data.uploadId
Check "objectKey 指向该视频的原片" ($s.data.objectKey -eq "video/original/$id/source.mp4") $s.data.objectKey
$uploadId = $s.data.uploadId

Write-Host "`n[4] 分片大小不合法应被拒 60005" -ForegroundColor Cyan
# part#2 不是末片，必须正好 5MiB；这里塞 1MiB
$bad = Join-Path $env:TEMP ("bad-" + [guid]::NewGuid().ToString("N") + ".bin")
Split-Part $all 0 (1 * $mib) $bad | Out-Null
$r = Put-Bin "/api/v1/videos/$id/upload/parts/2" $bad $uid
Check "非末片大小不符被拒 60005" ($r -like '*"code":60005*') $r
$r = Put-Bin "/api/v1/videos/$id/upload/parts/4" $bad $uid
Check "partNumber 越界被拒 60005" ($r -like '*"code":60005*') $r
Remove-Item $bad -Force

Write-Host "`n[5] 上传 part#1 / part#2" -ForegroundColor Cyan
$p1 = (Put-Bin "/api/v1/videos/$id/upload/parts/1" $partFiles[0] $uid) | ConvertFrom-Json
Check "part#1 上传成功" ($p1.code -eq 0) ($p1 | ConvertTo-Json -Compress)
Check "part#1 返回 ETag" (-not [string]::IsNullOrWhiteSpace($p1.data.part.etag)) $p1.data.part.etag
Check "已上传 1 片" ($p1.data.uploadedCount -eq 1) $p1.data.uploadedCount
Check "已上传字节 = 5MiB" ($p1.data.uploadedBytes -eq $partSize) $p1.data.uploadedBytes
Check "尚未完成" (-not $p1.data.completed) $p1.data.completed

$p2 = (Put-Bin "/api/v1/videos/$id/upload/parts/2" $partFiles[1] $uid) | ConvertFrom-Json
Check "part#2 上传成功" ($p2.code -eq 0) ($p2 | ConvertTo-Json -Compress)
Check "已上传 2 片" ($p2.data.uploadedCount -eq 2) $p2.data.uploadedCount

Write-Host "`n[6] 断点续传：重新查进度（模拟刷新页面）" -ForegroundColor Cyan
$st = (Get-J "/api/v1/videos/$id/upload/status" $uid) | ConvertFrom-Json
Check "status 成功" ($st.code -eq 0) ($st | ConvertTo-Json -Compress)
Check "已上传 2 片" ($st.data.uploadedCount -eq 2) $st.data.uploadedCount
$nums = @($st.data.uploadedParts | ForEach-Object { $_.partNumber }) -join ','
Check "已上传分片序号 = 1,2" ($nums -eq '1,2') $nums
Check "uploadId 未变" ($st.data.uploadId -eq $uploadId) $st.data.uploadId

Write-Host "`n[7] 重复 init 必须复用同一会话（不能把已传分片废掉）" -ForegroundColor Cyan
$s2 = (Post "/api/v1/videos/$id/upload/init" $body $uid) | ConvertFrom-Json
Check "重复 init 成功" ($s2.code -eq 0) ($s2 | ConvertTo-Json -Compress)
Check "复用同一 uploadId" ($s2.data.uploadId -eq $uploadId) $s2.data.uploadId
Check "已传分片仍在（2 片）" ($s2.data.uploadedCount -eq 2) $s2.data.uploadedCount

Write-Host "`n[8] 分片未齐就合并应被拒 60006" -ForegroundColor Cyan
$r = Post "/api/v1/videos/$id/upload/complete" '' $uid
Check "分片未齐被拒 60006" ($r -like '*"code":60006*') $r

Write-Host "`n[9] 补传 part#3 并合并" -ForegroundColor Cyan
$p3 = (Put-Bin "/api/v1/videos/$id/upload/parts/3" $partFiles[2] $uid) | ConvertFrom-Json
Check "part#3 上传成功（末片 1MiB）" ($p3.code -eq 0) ($p3 | ConvertTo-Json -Compress)
Check "已上传 3 片" ($p3.data.uploadedCount -eq 3) $p3.data.uploadedCount
Check "进度标记 completed" ($p3.data.completed -eq $true) $p3.data.completed
Check "已上传字节 = 11MiB" ($p3.data.uploadedBytes -eq $fileSize) $p3.data.uploadedBytes

$done = (Post "/api/v1/videos/$id/upload/complete" '' $uid) | ConvertFrom-Json
Check "合并成功" ($done.code -eq 0) ($done | ConvertTo-Json -Compress)
Check "状态转 TRANSCODING" ($done.data.status -eq 'TRANSCODING') $done.data.status
Check "返回 taskId" ($done.data.taskId -gt 0) $done.data.taskId
Check "sizeBytes = 11MiB（合并后按实际对象大小写入）" ($done.data.sizeBytes -eq $fileSize) $done.data.sizeBytes

Write-Host "`n[10] 合并后再查进度应 40404（会话已终结）" -ForegroundColor Cyan
$r = Get-J "/api/v1/videos/$id/upload/status" $uid
Check "会话已终结 40404" ($r -like '*"code":40404*') $r

Write-Host "`n[11] 等转码完成（mock driver）" -ForegroundColor Cyan
$status = ""; $info = $null
for ($i = 0; $i -lt 40; $i++) {
    $t = (Get-J "/api/v1/videos/$id/transcode" $uid) | ConvertFrom-Json
    if ($t.code -ne 0) { $status = "HTTP-" + $t.code; $info = $t; break }
    $info = $t.data
    $status = $t.data.status
    if ($status -notin @('PENDING', 'RUNNING', 'PROCESSING')) { break }
    Start-Sleep -Milliseconds 250
}
Write-Host ("  taskStatus=" + $status + " progress=" + $info.progress)
Check "转码成功" ($status -in @('SUCCEEDED', 'SUCCESS', 'READY')) ("taskStatus=$status")
$d = ((Get-J "/api/v1/videos/$id" $uid) | ConvertFrom-Json).data
Check "视频状态 READY" ($d.status -eq 'READY') $d.status
Check "playbackUrl 已写入" (-not [string]::IsNullOrWhiteSpace($d.playbackUrl)) $d.playbackUrl

Write-Host "`n[12] abort：取消上传并回收分片" -ForegroundColor Cyan
$v2 = (Post "/api/v1/videos" '{"title":"chunked abort e2e"}' $uid) | ConvertFrom-Json
$id2 = $v2.data.videoId
$body2 = '{"filename":"big.mp4","fileSize":' + $fileSize + '}'
$a1 = (Post "/api/v1/videos/$id2/upload/init" $body2 $uid) | ConvertFrom-Json
Check "第二个视频 init 成功" ($a1.code -eq 0) ($a1 | ConvertTo-Json -Compress)
$r = Put-Bin "/api/v1/videos/$id2/upload/parts/1" $partFiles[0] $uid
Check "传 1 片" ($r -like '*"code":0*') $r
$ab = (curl.exe -s -X DELETE "$base/api/v1/videos/$id2/upload" -H "X-User-Id: $uid")
Check "取消成功 aborted=true" ($ab -like '*"aborted":true*') $ab
$r = Get-J "/api/v1/videos/$id2/upload/status" $uid
Check "取消后查进度 40404" ($r -like '*"code":40404*') $r
$r = Put-Bin "/api/v1/videos/$id2/upload/parts/1" $partFiles[0] $uid
Check "取消后传分片 40404" ($r -like '*"code":40404*') $r

Write-Host "`n[13] 权限与参数校验" -ForegroundColor Cyan
$r = Get-J "/api/v1/videos/$id/upload/status"
Check "未登录 40101" ($r -like '*"code":40101*') $r
$r = Get-J "/api/v1/videos/$id/upload/status" "u-9999"
Check "非属主 40301" ($r -like '*"code":40301*') $r

$v3 = (Post "/api/v1/videos" '{"title":"chunked bad param"}' $uid) | ConvertFrom-Json
$id3 = $v3.data.videoId
$r = Post "/api/v1/videos/$id3/upload/init" '{"filename":"a.mp4","fileSize":0}' $uid
Check "fileSize=0 被拒 40001" ($r -like '*"code":40001*') $r
$r = Post "/api/v1/videos/$id3/upload/init" '{"filename":"a.exe","fileSize":1048576}' $uid
Check "非法扩展名被拒 60001" ($r -like '*"code":60001*') $r
$r = Post "/api/v1/videos/$id3/upload/init" ('{"filename":"a.mp4","fileSize":1048576,"partSize":1048576}') $uid
Check "partSize < 5MiB 被拒 60005" ($r -like '*"code":60005*') $r
$r = Get-J "/api/v1/videos/$id3/upload/status" $uid
Check "未 init 就查进度 40404" ($r -like '*"code":40404*') $r

# 清理本地临时文件
Remove-Item $src -Force -ErrorAction SilentlyContinue
foreach ($f in $partFiles) { Remove-Item $f -Force -ErrorAction SilentlyContinue }

Write-Host "`n========== 结果：通过 $script:pass 项，失败 $script:fail 项 ==========" -ForegroundColor Cyan
if ($script:fail -gt 0) { exit 1 }
