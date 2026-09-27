#Requires -Version 5.1
<#
.SYNOPSIS
    把「本地 9 个独立仓库」聚合成一份快照，推送到 GitHub 上唯一的**总仓库**。

.DESCRIPTION
    需求是「远端只有一个总仓库，本地 8 个服务各自一个仓库各自管理」。
    这件事不能把根仓库直接改造成总仓库，原因是 git 的硬限制：
    只要子目录里存在 .git，父仓库就只能记录一个 gitlink（commit 指针），
    clone 下来是 8 个空目录（git add -f 也无效，逐文件 add 会静默失败）。

    所以本脚本的做法是：
      1. 把工作区（不含任何 .git / 构建产物）镜像复制到 $PublishDir
      2. 在 $PublishDir 里写一份「聚合专用」的 .gitignore（不再排除 8 个服务目录）
      3. 生成 AGGREGATE.md，记录快照时各仓库的 commit / tag，作为版本锚点
      4. 在 $PublishDir 里 git init/add/commit/push 到远端唯一仓库

    本脚本**幂等、可反复运行**：
      每次运行 = 一次快照。远端仓库不存在会自动创建。
      本地 8 个仓库的历史不受任何影响，仍各自独立管理。

.PARAMETER PublishDir
    聚合快照目录。默认是工作区的同级目录 service-all（放在工作区外，
    避免 robocopy 自我递归，也避免根仓库需要额外忽略它）。

.PARAMETER RepoName
    远端总仓库名，默认 service-all。

.PARAMETER Owner
    GitHub 账号。默认从 git config 的 noreply 邮箱推断。

.PARAMETER Visibility
    private（默认）或 public。配置里有明文数据库口令，默认 private。

.PARAMETER Message
    覆盖提交信息。默认自动生成 sync: 聚合快照 <时间>。

.PARAMETER KeepHistory
    不重建聚合仓库的历史（默认每次都是「增量提交」，本来也是增量）。
    加 -Fresh 才会清空聚合仓库的 .git 重新开始。

.PARAMETER Fresh
    删掉聚合目录里的 .git 重新初始化（慎用，会丢失聚合仓库的提交历史）。

.PARAMETER DryRun
    只打印将要执行的动作，不复制、不提交、不推送。

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File tools/publish-all.ps1 -DryRun

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File tools/publish-all.ps1
#>
[CmdletBinding()]
param(
    [string] $PublishDir,
    [string] $RepoName = 'service-all',
    [string] $Owner,
    [ValidateSet('private', 'public')] [string] $Visibility = 'private',
    [string] $Message,
    [switch] $Fresh,
    [switch] $DryRun
)

# 不能用 'Stop'：gh/git 写 stderr 会被 PowerShell 5.1 包成 ErrorRecord 并直接终止脚本。
$ErrorActionPreference = 'Continue'

# 把文本统一成 LF 换行。本机 core.autocrlf=true，聚合仓库又声明了 eol=lf，
# 若生成的文件带 CRLF，每次 add 都会刷 "CRLF will be replaced by LF" 警告。
# 而 here-string 的内容继承脚本自身的换行符，WriteAllLines 用的是 Environment.NewLine，
# 所以生成的文本统一走这个函数收口。
function ConvertTo-LfText {
    param([string] $Text)
    return (($Text -split "`r?`n") -join "`n")
}

# ---------------------------------------------------------------- 路径与账号
$root = Split-Path $PSScriptRoot -Parent
if (-not $PublishDir) { $PublishDir = Join-Path (Split-Path $root -Parent) $RepoName }

if (-not $Owner) {
    $email = git config --global user.email
    if ($email -match '^\d+\+(.+)@users\.noreply\.github\.com$') { $Owner = $Matches[1] }
}
if (-not $Owner) { throw '无法从 git config 推断 GitHub 用户名，请显式传 -Owner' }

# 聚合进总仓库的东西 = 工作区根内容 + 8 个服务，所以工作区根必须是 git 仓库
$services = @(
    'auth-service', 'gateway-service', 'user-service', 'product-service', 'order-service',
    'video-service-go', 'live-service-go', 'danmaku-service-go'
)

if (-not (Test-Path $root)) { throw "工作区根不存在: $root" }

# 聚合目录不能在工作区内部：否则 robocopy 会自我递归，
# 而且工作区根仓库还得额外忽略它。
$rootFull = (Resolve-Path $root).Path.TrimEnd('\')
$publishFull = [System.IO.Path]::GetFullPath($PublishDir).TrimEnd('\')
if ($publishFull -eq $rootFull -or $publishFull.StartsWith($rootFull + '\')) {
    throw "聚合目录必须放在工作区之外。当前值: $publishFull"
}
$PublishDir = $publishFull

Write-Host "工作区      : $root" -ForegroundColor Cyan
Write-Host "聚合目录    : $PublishDir" -ForegroundColor Cyan
Write-Host "远端仓库    : https://github.com/$Owner/$RepoName" -ForegroundColor Cyan
Write-Host "可见性      : $Visibility" -ForegroundColor Cyan
if ($DryRun) { Write-Host "模式        : DryRun（不产生任何本地/远端改动）" -ForegroundColor Yellow }
Write-Host ''

# ---------------------------------------------------------------- 收集版本锚点
function Get-RepoVersion {
    param([string] $Dir)
    if (-not (Test-Path (Join-Path $Dir '.git'))) { return $null }
    $sha = (git -C $Dir rev-parse --short HEAD 2>$null)
    if (-not $sha) { return $null }
    return [pscustomobject]@{
        Branch   = (git -C $Dir rev-parse --abbrev-ref HEAD 2>$null).Trim()
        Commit   = $sha.Trim()
        Tags     = ((git -C $Dir tag --points-at HEAD 2>$null) -join ', ')
        Dirty    = [bool](git -C $Dir status --porcelain 2>$null)
    }
}

Write-Host '--- 收集各仓库版本锚点 ---' -ForegroundColor Cyan
$versions = [ordered]@{}
$versions['(工作区根 / infra)'] = Get-RepoVersion $root
foreach ($s in $services) { $versions[$s] = Get-RepoVersion (Join-Path $root $s) }
foreach ($k in $versions.Keys) {
    $v = $versions[$k]
    $txt = if ($v) { "$($v.Commit)  $($v.Branch)  tags=$($v.Tags)$(if ($v.Dirty) { '  [有未提交改动]' })" } else { '(不是 git 仓库)' }
    Write-Host ("  " + $k.PadRight(22) + $txt)
}
Write-Host ''

# ---------------------------------------------------------------- 1) 镜像复制
# /MIR = /E + /PURGE：目标里多出来的文件会被删除，保证快照与工作区一致
# /XD .git    排除所有 .git（这既是「不复制」也是「不删除」目标的 .git）
# /XD target 等  排除构建产物与运行期数据，它们本来也被各级 .gitignore 忽略
$xd = @('.git', 'target', 'logs', 'bin', '.work', 'node_modules', 'dist', 'build', 'out', '.idea', '.gradle')
$xf = @('*.log', '*.tmp', '*.bak', '*.swp', 'Thumbs.db', 'Desktop.ini', '.DS_Store')
$rcArgs = @($root, $PublishDir, '/MIR',
    '/XD') + $xd + @('/XF') + $xf + @(
    '/NFL', '/NDL', '/NJH', '/NJS', '/NP', '/R:1', '/W:1')

if ($DryRun) {
    Write-Host '--- DryRun：将执行 robocopy（不实际执行）---' -ForegroundColor Yellow
    Write-Host ("  robocopy " + ($rcArgs -join ' '))
}
else {
    if ($Fresh -and (Test-Path (Join-Path $PublishDir '.git'))) {
        Write-Host '--- -Fresh：清空聚合仓库的 git 历史 ---' -ForegroundColor Yellow
        Remove-Item (Join-Path $PublishDir '.git') -Recurse -Force
    }

    Write-Host '--- 1/4 镜像复制工作区 ---' -ForegroundColor Cyan
    if (-not (Test-Path $PublishDir)) { New-Item -ItemType Directory -Path $PublishDir -Force | Out-Null }
    & robocopy @rcArgs | Out-Null
    $rc = $LASTEXITCODE
    Write-Host ("  robocopy 退出码 = $rc" + $(if ($rc -ge 8) { '  ← 失败（>=8）' } else { '  （0-7 均为成功）' }))
    if ($rc -ge 8) { throw "robocopy 失败，退出码 $rc" }

    # ------------------------------------------------------------ 2) 聚合专用 .gitignore
    Write-Host '--- 2/4 生成聚合专用 .gitignore ---' -ForegroundColor Cyan
    $gitignore = @'
# ===========================================================================
# 本文件由 tools/publish-all.ps1 自动生成，请勿手工编辑
#
# 这是「聚合总仓库」的快照副本，与工作区根仓库的 .gitignore 不同：
# 工作区根仓库刻意排除 8 个服务目录（它们各自是独立仓库），
# 而聚合仓库必须收录它们，所以这里没有 /xxx-service/ 这类排除规则。
#
# 各服务自己的 .gitignore 已随目录一起复制过来，会在子目录层级继续生效
# （git 会读取每一级目录的 .gitignore）。
# ===========================================================================

# ---------------------------------------------------------------- 本地环境变量
deploy/.env
.env
.env.local

# ---------------------------------------------------------------- 构建产物 / 运行期数据
target/
logs/
bin/
.work/
node_modules/
dist/
build/
out/

# ---------------------------------------------------------------- 临时文件
*.tmp
*.bak
*~
*.log

# ---------------------------------------------------------------- 编辑器 / IDE
.idea/
*.iml
*.swp
*.swo

# ---------------------------------------------------------------- 操作系统
Thumbs.db
Desktop.ini
.DS_Store
'@
    # 无 BOM：.gitignore 带 BOM 会让第一条规则失效
    [System.IO.File]::WriteAllText((Join-Path $PublishDir '.gitignore'), (ConvertTo-LfText $gitignore), [System.Text.UTF8Encoding]::new($false))
    Write-Host '  写入 .gitignore'

    # 统一换行符为 LF。本机 core.autocrlf=true 时，若不声明，每次 add 都会对几百个
    # 文件刷一行 "LF will be replaced by CRLF" 警告，噪音很大且没有意义。
    $gitattributes = @'
# 本文件由 tools/publish-all.ps1 自动生成，请勿手工编辑
# 统一以 LF 入库并在检出时保持 LF，避免 Windows 上 core.autocrlf 带来的换行符抖动。
* text=auto eol=lf
'@
    [System.IO.File]::WriteAllText((Join-Path $PublishDir '.gitattributes'), (ConvertTo-LfText $gitattributes), [System.Text.UTF8Encoding]::new($false))
    Write-Host '  写入 .gitattributes'

    # ------------------------------------------------------------ 3) AGGREGATE.md
    Write-Host '--- 3/4 生成 AGGREGATE.md（版本锚点）---' -ForegroundColor Cyan
    $now = Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz'
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add('# 聚合总仓库说明')
    $lines.Add('')
    $lines.Add('本仓库由 `tools/publish-all.ps1` **自动生成**，是 9 个独立本地仓库的一份只读快照，')
    $lines.Add('目的是「远端只有一个总仓库、方便浏览与分享」，**不是开发仓库**。')
    $lines.Add('')
    $lines.Add('> 开发与完整提交历史请使用本地各自的仓库：')
    $lines.Add('> 8 个服务仓库 + 1 个工作区根仓库（infra：`deploy/`、`tools/`、跨服务文档）。')
    $lines.Add('')
    # 注意：这里全部用单引号字符串拼接。
    # 双引号字符串里 ` 是转义符，`$ 会变成字面量 $，`" 会变成字面量引号并导致字符串不闭合，
    # 而我们要输出的恰恰是 markdown 的反引号，所以只能用单引号（单引号内 ` 是普通字符）。
    $lines.Add('快照时间：`' + $now + '`')
    $lines.Add('')
    $lines.Add('## 快照时的各仓库版本')
    $lines.Add('')
    $lines.Add('| 仓库 | 分支 | commit | tag | 备注 |')
    $lines.Add('| --- | --- | --- | --- | --- |')
    foreach ($k in $versions.Keys) {
        $v = $versions[$k]
        if ($v) {
            $name = if ($k -eq '(工作区根 / infra)') { '工作区根（infra）' } else { $k }
            $note = if ($v.Dirty) { '快照时存在未提交改动' } else { '' }
            $lines.Add('| ' + $name + ' | ' + $v.Branch + ' | `' + $v.Commit + '` | ' + $v.Tags + ' | ' + $note + ' |')
        }
        else {
            $lines.Add("| $k | - | - | - | 不是 git 仓库 |")
        }
    }
    $lines.Add('')
    $lines.Add('## 如何更新本仓库')
    $lines.Add('')
    $lines.Add('在本地工作区执行（可反复运行，每次运行即一次快照）：')
    $lines.Add('')
    $lines.Add('```powershell')
    $lines.Add('powershell -NoProfile -ExecutionPolicy Bypass -File tools\publish-all.ps1')
    $lines.Add('```')
    $lines.Add('')
    $lines.Add('脚本会：镜像复制工作区（排除 `.git` 与构建产物）→ 重新生成本文件 → 提交 → 推送。')
    $lines.Add('')
    $lines.Add('## 目录构成')
    $lines.Add('')
    $lines.Add('- `deploy/` —— 部署编排（compose / 环境变量模板 / 数据库基线 / K8s / SRS 配置）')
    $lines.Add('- 8 个服务目录 —— `auth` / `gateway` / `user` / `product` / `order`（Java）、`video` / `live` / `danmaku`（Go）')
    $lines.Add('- 根目录 `*.md` —— 跨服务设计文档、接口测试文档、公共约定与问题记录')
    $lines.Add('')
    # WriteAllLines 用的是 Environment.NewLine（Windows 下 = CRLF），这里改用 LF 写出
    [System.IO.File]::WriteAllText((Join-Path $PublishDir 'AGGREGATE.md'), ((ConvertTo-LfText ($lines -join "`n")) + "`n"), [System.Text.UTF8Encoding]::new($false))
    Write-Host '  写入 AGGREGATE.md'

    # ------------------------------------------------------------ 4) 提交并推送
    Write-Host '--- 4/4 提交并推送到远端总仓库 ---' -ForegroundColor Cyan
    Push-Location $PublishDir
    try {
        if (-not (Test-Path '.git')) {
            git init -b main 2>&1 | Out-Null
            Write-Host '  已初始化聚合仓库'
        }

        $remoteUrl = "https://github.com/$Owner/$RepoName.git"
        if ((git remote) -contains 'origin') { git remote set-url origin $remoteUrl }
        else { git remote add origin $remoteUrl }

        git add -A
        if (git status --porcelain) {
            if (-not $Message) { $Message = "sync: 聚合快照 $now" }
            $body = New-Object System.Collections.Generic.List[string]
            $body.Add($Message)
            $body.Add('')
            $body.Add('各仓库快照版本：')
            foreach ($k in $versions.Keys) {
                if ($versions[$k]) { $body.Add("- $k = $($versions[$k].Commit)") }
            }
            $msgFile = Join-Path $env:TEMP 'publish-all-msg.txt'
            [System.IO.File]::WriteAllText($msgFile, ((ConvertTo-LfText ($body -join "`n")) + "`n"), [System.Text.UTF8Encoding]::new($false))
            git commit -F $msgFile 2>&1 | ForEach-Object { Write-Host "  $_" }
            Remove-Item $msgFile -Force
        }
        else {
            Write-Host '  内容无变化，跳过提交'
        }

        # 远端不存在则创建
        gh repo view "$Owner/$RepoName" --json name 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Host '  远端总仓库不存在，创建中 ...'
            gh repo create "$Owner/$RepoName" "--$Visibility" 2>&1 | ForEach-Object { Write-Host "    $_" }
            # 探测可能瞬时失败，再确认一次
            gh repo view "$Owner/$RepoName" --json name 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) { throw "创建远端仓库失败: $Owner/$RepoName" }
        }

        $branch = (git rev-parse --abbrev-ref HEAD).Trim()
        git push -u origin $branch 2>&1 | ForEach-Object { Write-Host "  $_" }
        if ($LASTEXITCODE -ne 0) { throw 'git push 失败' }
    }
    finally {
        Pop-Location
    }
}

Write-Host ''
if ($DryRun) {
    Write-Host 'DryRun 结束：未做任何本地/远端改动。' -ForegroundColor Yellow
}
else {
    Write-Host "完成：https://github.com/$Owner/$RepoName" -ForegroundColor Green
}

# 显式退出码：PowerShell 5.1 下 -File 的退出码会受前面原生命令的 $LASTEXITCODE 影响
# （例如 gh repo view 探测「不存在」时会留下 1），所以这里明确收口。
exit 0
