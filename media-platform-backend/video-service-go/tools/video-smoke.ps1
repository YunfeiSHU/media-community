# video-service-go 端到端冒烟（需 video-service 已在 9001 运行，MinIO 可用）
# 用法：powershell -NoProfile -ExecutionPolicy Bypass -File tools/video-smoke.ps1
#
# 契约要点（读源码确认过，别按直觉猜）：
#  1. 创建视频只建「元数据」，status=DRAFT；上传原片 POST /:id/upload（multipart，字段名 file），
#     上传后 status=TRANSCODING 并投递 video.uploaded，由 transcode_worker 异步消费。
#  2. transcoder.driver 默认 mock，所以假 MP4 也能走完「转码」链路 →
#     轮询 /:id/transcode 直到成功，再发布（CanPublish = READY|OFFLINE）。
#  3. 错误码不连续：60001=文件类型不支持，60002=文件过大，60003=状态不允许，60004=任务提交失败。
#  4. 列表项字段是 id（VideoBrief），详情/播放才是 videoId。

$ErrorActionPreference = "Stop"
$base = "http://localhost:9001"
$uid = "u-6001"

$script:pass = 0
$script:fail = 0
function Check([string]$name, [bool]$ok, [string]$detail) {
    if ($ok) { $script:pass++; Write-Host ("  [PASS] " + $name) -ForegroundColor Green }
    else { $script:fail++; Write-Host ("  [FAIL] " + $name + "  -> " + $detail) -ForegroundColor Red }
}
function New-BodyFile([string]$json) {
    $f = Join-Path $env:TEMP ("video-smoke-" + [guid]::NewGuid().ToString("N") + ".json")
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

Write-Host "`n[1] 健康检查" -ForegroundColor Cyan
$h = (Get-J "/healthz") | ConvertFrom-Json
Check "healthz UP" ($h.data.status -eq 'UP') ($h | ConvertTo-Json -Compress)
$r = (Get-J "/readyz") | ConvertFrom-Json
Check "readyz UP" ($r.data.status -eq 'UP') ($r | ConvertTo-Json -Compress)
foreach ($c in $r.data.checks) { Write-Host ("  check " + $c.name + " = " + $c.status + " (" + $c.detail + ")") }

Write-Host "`n[2] 未登录创建视频应 401" -ForegroundColor Cyan
$r = Post "/api/v1/videos" '{"title":"x"}'
Check "未登录被拒 40101" ($r -like '*"code":40101*') $r

Write-Host "`n[3] 分类接口（公开）" -ForegroundColor Cyan
$r = Get-J "/api/v1/categories"
Check "分类列表可访问" ($r -like '*"code":0*') $r

Write-Host "`n[4] 创建视频草稿" -ForegroundColor Cyan
$r = Post "/api/v1/videos" '{"title":"e2e video","description":"smoke","tags":["smoke"]}' $uid
$v = ($r | ConvertFrom-Json)
Check "创建成功" ($v.code -eq 0) $r
Check "初始状态 DRAFT" ($v.data.status -eq 'DRAFT') $v.data.status
$id = $v.data.videoId
Write-Host ("  videoId=$id")

Write-Host "`n[5] 详情与列表（注意视角：草稿只对属主可见）" -ForegroundColor Cyan
# 详情/播放入口必须带 X-User-Id，否则会被当作游客：非 PUBLISHED 的视频直接 40401（不泄露存在性）
$d = ((Get-J "/api/v1/videos/$id" $uid) | ConvertFrom-Json).data
Check "详情 title 正确" ($d.title -eq 'e2e video') $d.title
Check "详情 ownerId 正确" ($d.ownerId -eq $uid) $d.ownerId
Check "详情 tags 正确" (@($d.tags) -contains 'smoke') ($d.tags -join ',')
Check "详情状态 DRAFT" ($d.status -eq 'DRAFT') $d.status
Check "详情字段名是 id" ($d.id -eq $id) $d.id
# 列表项字段是 id（VideoBrief），不是 videoId；草稿必须用 mine=true 才看得到
$mine = ((Get-J "/api/v1/videos?mine=true&page=1&pageSize=5" $uid) | ConvertFrom-Json).data
Check "我的列表包含草稿" (@($mine.items | Where-Object { $_.id -eq $id }).Count -ge 1) "count=$(@($mine.items).Count)"
# 公开列表（游客）不应包含未发布视频
$pub = ((Get-J "/api/v1/videos?page=1&pageSize=20") | ConvertFrom-Json).data
Check "公开列表不含草稿" (@($pub.items | Where-Object { $_.id -eq $id }).Count -eq 0) "count=$(@($pub.items).Count)"
Check "游客看草稿详情 40401" ((Get-J "/api/v1/videos/$id") -like '*"code":40401*') (Get-J "/api/v1/videos/$id")

Write-Host "`n[6] 非属主不能删我的视频" -ForegroundColor Cyan
$r = (& curl.exe -s -X DELETE "$base/api/v1/videos/$id" -H "X-User-Id: u-9999")
Check "非属主删除被拒 40301" ($r -like '*"code":40301*') $r

Write-Host "`n[7] 上传原片（真实 MinIO PutObject）" -ForegroundColor Cyan
$tmp = Join-Path $env:TEMP ("fake-" + [guid]::NewGuid().ToString("N") + ".mp4")
# 3 字节假文件：扩展名合法 + MinIO 可写就能走通上传链路（transcoder=mock 不真的解码）
[System.IO.File]::WriteAllBytes($tmp, [byte[]](0x00, 0x00, 0x01))
$r = (& curl.exe -s -X POST "$base/api/v1/videos/$id/upload" -H "X-User-Id: $uid" -F "file=@$tmp")
Remove-Item $tmp -Force
$up = $r | ConvertFrom-Json
Check "上传成功" ($up.code -eq 0) $r
Check "状态转 TRANSCODING" ($up.data.status -eq 'TRANSCODING') $up.data.status
Check "返回 taskId" ($up.data.taskId -gt 0) $up.data.taskId
Check "记录文件大小 3 字节" ($up.data.sizeBytes -eq 3) $up.data.sizeBytes

Write-Host "`n[8] 上传非法扩展名应被拒 60001" -ForegroundColor Cyan
$tmp2 = Join-Path $env:TEMP ("fake-" + [guid]::NewGuid().ToString("N") + ".exe")
[System.IO.File]::WriteAllBytes($tmp2, [byte[]](0x00))
$r = (& curl.exe -s -X POST "$base/api/v1/videos/$id/upload" -H "X-User-Id: $uid" -F "file=@$tmp2")
Remove-Item $tmp2 -Force
Check "非法类型被拒 60001" ($r -like '*"code":60001*') $r

Write-Host "`n[9] 等 MQ 消费者把转码跑完（mock driver）" -ForegroundColor Cyan
$status = ""
$info = $null
for ($i = 0; $i -lt 40; $i++) {
    $t = (Get-J "/api/v1/videos/$id/transcode" $uid) | ConvertFrom-Json
    if ($t.code -ne 0) { $status = "HTTP-" + $t.code; $info = $t; break }
    $info = $t.data
    $status = $t.data.status
    if ($status -notin @('PENDING', 'RUNNING', 'PROCESSING')) { break }
    Start-Sleep -Milliseconds 250
}
$msg = if ($info.message) { $info.message } else { "" }
Write-Host ("  taskStatus=" + $status + " progress=" + $info.progress + " profiles=" + ($info.profiles -join '/') + " msg=" + $msg)
Check "转码任务成功结束" ($status -in @('SUCCEEDED', 'SUCCESS', 'READY')) ("taskStatus=$status msg=$msg")
$d = ((Get-J "/api/v1/videos/$id" $uid) | ConvertFrom-Json).data
Check "视频状态变 READY" ($d.status -eq 'READY') $d.status
Check "READY 已写入 playbackUrl" (-not [string]::IsNullOrWhiteSpace($d.playbackUrl)) $d.playbackUrl

Write-Host "`n[10] 未发布时游客拿不到播放信息（40401，不泄露存在性）" -ForegroundColor Cyan
# 注意：属主自己可以拿自己的 READY 视频玩（归属即可见），
# 所以这里必须用游客视角断言，否则会误判。
$r = Get-J "/api/v1/videos/$id/play"
Check "游客播放未发布视频被拒 40401" ($r -like '*"code":40401*') $r
$r = Get-J "/api/v1/videos/$id/play" $uid
Check "属主可预看自己的 READY 视频" ($r -like '*"code":0*') $r

Write-Host "`n[11] 发布" -ForegroundColor Cyan
$r = Post "/api/v1/videos/$id/publish" "" $uid
$p = $r | ConvertFrom-Json
Check "发布成功" ($p.code -eq 0) $r
Check "状态变 PUBLISHED" ($p.data.status -eq 'PUBLISHED') $p.data.status

Write-Host "`n[12] 游客取播放地址" -ForegroundColor Cyan
$play = ((Get-J "/api/v1/videos/$id/play") | ConvertFrom-Json).data
Check "播放 videoId 正确" ($play.videoId -eq $id) $play.videoId
Check "播放地址非空" (-not [string]::IsNullOrWhiteSpace($play.playbackUrl)) $play.playbackUrl
Check "含多码率 sources" (@($play.sources).Count -ge 1) "count=$(@($play.sources).Count)"
foreach ($s in $play.sources) { Write-Host ("  source " + $s.profile + " -> " + $s.url) }

Write-Host "`n[13] 下架 → 游客不可见" -ForegroundColor Cyan
$r = Post "/api/v1/videos/$id/offline" "" $uid
$o = $r | ConvertFrom-Json
Check "下架成功" ($o.code -eq 0) $r
Check "状态变 OFFLINE" ($o.data.status -eq 'OFFLINE') $o.data.status
$r = Get-J "/api/v1/videos/$id/play"
Check "下架后游客播放被拒 40401" ($r -like '*"code":40401*') $r
$pub = ((Get-J "/api/v1/videos?page=1&pageSize=20") | ConvertFrom-Json).data
Check "下架后公开列表不含它" (@($pub.items | Where-Object { $_.id -eq $id }).Count -eq 0) "count=$(@($pub.items).Count)"

Write-Host "`n[14] 重新上架（OFFLINE → PUBLISHED）" -ForegroundColor Cyan
$r = Post "/api/v1/videos/$id/publish" "" $uid
Check "重新发布成功" (($r | ConvertFrom-Json).code -eq 0) $r
Check "游客恢复可见" ((Get-J "/api/v1/videos/$id/play") -like '*"code":0*') (Get-J "/api/v1/videos/$id/play")

Write-Host "`n[15] 清理" -ForegroundColor Cyan
$r = (& curl.exe -s -X DELETE "$base/api/v1/videos/$id" -H "X-User-Id: $uid")
Check "删除成功" ($r -like '*"code":0*') $r
$r = Get-J "/api/v1/videos/$id" $uid
Check "删除后详情 40401" ($r -like '*"code":40401*') $r

Write-Host "`n========== 结果：通过 $script:pass 项，失败 $script:fail 项 ==========" -ForegroundColor Cyan
if ($script:fail -gt 0) { exit 1 }
Write-Host "全部通过" -ForegroundColor Green
