#Requires -Version 5.1
<#
.SYNOPSIS
    把「本地 9 个独立仓库」聚合推送到 GitHub 上唯一的**总仓库**，并保留各仓库真实的提交信息。

.DESCRIPTION
    需求是「远端只建立一个总仓库，本地 8 个服务各自一个仓库在本地管理」。
    这件事不能把根仓库直接改造成总仓库，原因是 git 的硬限制：
    只要子目录里存在 .git，父仓库就只能记录一个 gitlink（commit 指针），
    clone 下来是 8 个空目录（git add -f 也无效，逐文件 add 会静默失败）。

    所以本脚本的做法是：在 $PublishDir 里重建一个「聚合仓库」，
    它收录工作区根内容 + 8 个服务目录，再推送到远端唯一仓库。

    ============================ 提交信息如何保留 ============================

    早期版本是「全量镜像 + 一次性 git add -A + 单条 `sync: 聚合快照 <时间>`」，
    结果是远端 git log 里只剩一串时间戳，各服务仓库精心写的提交信息全部丢失。
    现在改成**逐条重放**：

      1. 从聚合仓库上一版 AGGREGATE.md 里读出 9 个仓库各自的「上次快照锚点」
      2. 对每个仓库算出 锚点..HEAD 之间的新提交
      3. 把所有仓库的新提交**按作者时间排序**（还原真实的时间线交错）
      4. 逐条重放：用 `git worktree add --detach <sha>` 取出该 revision 的完整树，
         镜像到聚合仓库的对应前缀下，再用**原文**提交（主题、正文、作者、时间都保留），
         只在正文末尾追加两行来源标注：
             Source-Repo: gateway-service
             Source-Commit: afd0344
      5. 最后同步各仓库**未提交**的工作树改动（如果有），每个仓库一条 snapshot 提交
      6. 刷新 .gitignore / .gitattributes / AGGREGATE.md（含下一轮要用的锚点），一条 chore 提交
      7. 推送

    关键性质：
      * **锚点**保证增量 —— 同一批提交只会被重放一次，反复运行不会产生重复历史
      * **保留现有远端历史** —— 已有提交原样留在前面，新提交追加在后，无需 force push
      * **提交信息零损失** —— 主题 / 正文 / 作者 / 时间与源仓库一致（仅追加 2 行 source 标注）
      * 若某仓库**找不到可用锚点**（首次运行且聚合仓库里没有 AGGREGATE.md），
        该仓库退化为「整体快照一条提交」，而不是把全部历史重放一遍，避免内容重复

    ============================== 编码注意 ==============================

    PowerShell 5.1 的 [Console]::OutputEncoding 默认是系统 ANSI 代码页（本机 CP936/gb2312），
    而 git 输出的是 UTF-8。若不干预，`& git log --format=%s` 抓到手的中文是乱码字符串，
    再写进提交信息文件就会把中文提交信息彻底写坏（实测 67 字节 → 81 字节的 mojibake）。
    所以脚本开头会强制把控制台输出编码设为 UTF-8。改写前务必保留这段。

.PARAMETER PublishDir
    聚合快照目录。默认是工作区的同级目录，名字与 RepoName 相同（放在工作区外，
    避免 robocopy 自我递归，也避免根仓库需要额外忽略它）。

.PARAMETER RepoName
    远端总仓库名，默认 media-community-backend。

.PARAMETER Owner
    GitHub 账号。默认从 git config 的 noreply 邮箱推断。

.PARAMETER Visibility
    private（默认）或 public。配置里有明文数据库口令，默认 private。

.PARAMETER Message
    覆盖最后那条「快照元数据」提交的信息（默认 `chore: 刷新聚合快照元数据 <时间>`）。

.PARAMETER ReplayAll
    忽略锚点，把 9 个仓库的**全部历史**从头逐条重放；推送时自动改用 --force。
    配合 -Fresh 使用可以重建出一份完整、干净的镜像历史（会重写远端，慎用）。

.PARAMETER Fresh
    删掉聚合目录里的 .git 重新初始化，并忽略已有锚点（各仓库退化为整体快照）。

.PARAMETER DryRun
    只打印将要执行的动作（含完整重放清单），不复制、不提交、不推送。

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File tools/publish-all.ps1 -DryRun

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File tools/publish-all.ps1

.EXAMPLE
    # 重建完整镜像历史（会重写远端）
    powershell -NoProfile -ExecutionPolicy Bypass -File tools/publish-all.ps1 -ReplayAll -Fresh
#>
[CmdletBinding()]
param(
    [string] $PublishDir,
    [string] $RepoName = 'media-community-backend',
    [string] $Owner,
    [ValidateSet('private', 'public')] [string] $Visibility = 'private',
    [string] $Message,
    [switch] $ReplayAll,
    [switch] $Fresh,
    [switch] $DryRun
)

# 不能用 'Stop'：gh/git 写 stderr 会被 PowerShell 5.1 包成 ErrorRecord 并直接终止脚本。
$ErrorActionPreference = 'Continue'

# ---------------------------------------------------------------- 输出编码
# 必须在任何 git 调用之前设置，否则读到的中文提交信息是乱码（见文件头说明）。
try {
    [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)
    $OutputEncoding = [System.Text.UTF8Encoding]::new($false)
}
catch {
    Write-Warning "设置 UTF-8 输出编码失败，中文提交信息可能乱码: $($_.Exception.Message)"
}

# ===========================================================================
# 通用工具
# ===========================================================================

# 把文本统一成 LF 换行。本机 core.autocrlf=true，聚合仓库又声明了 eol=lf，
# 若生成的文件带 CRLF，每次 add 都会刷 "CRLF will be replaced by LF" 警告。
# 而 here-string 的内容继承脚本自身的换行符，WriteAllLines 用的是 Environment.NewLine，
# 所以生成的文本统一走这个函数收口。
function ConvertTo-LfText {
    param([string] $Text)
    return (($Text -split "`r?`n") -join "`n")
}

# 写 UTF-8 无 BOM + LF 的文本文件
# （生成的 .gitignore / .gitattributes / 提交信息都要这样写；.gitignore 带 BOM 会让首条规则失效）
function Write-Utf8NoBom {
    param([string] $Path, [string] $Text)
    [System.IO.File]::WriteAllText($Path, (ConvertTo-LfText $Text), [System.Text.UTF8Encoding]::new($false))
}

# git 输出统一收口（stderr 丢弃），返回行数组
function Invoke-GitLines {
    param([string] $Dir, [string[]] $GitArgs)
    $out = & git -C $Dir @GitArgs 2>$null
    return @($out)
}

# 只取一行（rev-parse 这类）
function Get-GitScalar {
    param([string] $Dir, [string[]] $GitArgs)
    $lines = Invoke-GitLines -Dir $Dir -GitArgs $GitArgs
    if ($lines.Count -eq 0) { return $null }
    $v = "$($lines[0])".Trim()
    if ($v) { return $v }
    return $null
}

# ---------------------------------------------------------------- 路径与账号
$root = Split-Path $PSScriptRoot -Parent
if (-not $PublishDir) { $PublishDir = Join-Path (Split-Path $root -Parent) $RepoName }

if (-not $Owner) {
    $email = git config --global user.email
    if ($email -match '^\d+\+(.+)@users\.noreply\.github\.com$') { $Owner = $Matches[1] }
}
if (-not $Owner) { throw '无法从 git config 推断 GitHub 用户名，请显式传 -Owner' }

$serviceNames = @(
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

if ($Fresh -and -not $DryRun -and (Test-Path (Join-Path $PublishDir '.git'))) {
    Write-Host '-Fresh：清空聚合仓库的 git 历史' -ForegroundColor Yellow
    Remove-Item (Join-Path $PublishDir '.git') -Recurse -Force
}

# ---------------------------------------------------------------- 聚合单元定义
# 每个「单元」= 一个本地仓库 + 它在聚合仓库里的落地位置
#   工作区根（infra）→ 聚合仓库根目录（但排除 8 个服务目录）
#   8 个服务         → 聚合仓库下的同名子目录
$units = New-Object System.Collections.Generic.List[object]
$units.Add([pscustomobject]@{
        Key    = 'workspace-root'
        Name   = '工作区根（infra）'
        Dir    = $root
        Prefix = ''
        IsRoot = $true
    })
foreach ($s in $serviceNames) {
    $units.Add([pscustomobject]@{
            Key    = $s
            Name   = $s
            Dir    = (Join-Path $root $s)
            Prefix = $s
            IsRoot = $false
        })
}

Write-Host "工作区      : $root" -ForegroundColor Cyan
Write-Host "聚合目录    : $PublishDir" -ForegroundColor Cyan
Write-Host "远端仓库    : https://github.com/$Owner/$RepoName" -ForegroundColor Cyan
Write-Host "可见性      : $Visibility" -ForegroundColor Cyan
Write-Host "模式        : $(if ($ReplayAll) { 'ReplayAll（重放全部历史）' } else { '增量重放（按锚点）' })" -ForegroundColor Cyan
if ($DryRun) { Write-Host 'DryRun      : 不产生任何本地/远端改动' -ForegroundColor Yellow }
Write-Host ''

# ---------------------------------------------------------------- robocopy 排除项
# /MIR = /E + /PURGE：目标里多出来的文件会被删除，保证快照与源一致
# /XD 用**裸目录名**而不是绝对路径：robocopy 的 /XD 对源和目标同时生效，
#      只有裸名才能同时匹配两边（给绝对路径会导致目标侧的目录不被排除而被 /MIR 误删）。
# /XF 里带 .git 是因为 git worktree 会在工作树根放一个 **.git 文件**（不是目录），
#      /XD 只匹配目录，拦不住它，一旦复制进聚合仓库就会把仓库结构搞坏。
$xdCommon = @('.git', 'target', 'logs', 'bin', '.work', 'node_modules', 'dist', 'build', 'out', '.idea', '.gradle')
$xfCommon = @('*.log', '*.tmp', '*.bak', '*.swp', 'Thumbs.db', 'Desktop.ini', '.DS_Store', '.git')

# 根单元：额外排除 8 个服务目录（它们由各自的单元负责同步），
#        以及脚本自己生成的三个文件（否则会被根仓库的同名文件覆盖掉）
$xdRoot = $xdCommon + $serviceNames
$xfRoot = $xfCommon + @('.gitignore', '.gitattributes', 'AGGREGATE.md')

function Sync-UnitTree {
    param(
        [string] $From,
        [string] $To,
        [bool]   $IsRoot
    )
    $xd = if ($IsRoot) { $xdRoot } else { $xdCommon }
    $xf = if ($IsRoot) { $xfRoot } else { $xfCommon }
    if (-not (Test-Path $To)) { New-Item -ItemType Directory -Path $To -Force | Out-Null }
    $rcArgs = @($From, $To, '/MIR', '/XD') + $xd + @('/XF') + $xf + @('/NFL', '/NDL', '/NJH', '/NJS', '/NP', '/R:1', '/W:1')
    & robocopy @rcArgs | Out-Null
    return $LASTEXITCODE
}

# 单元在聚合仓库里的落地目录
function Get-UnitDest {
    param($Unit)
    if ($Unit.IsRoot) { return $PublishDir }
    return (Join-Path $PublishDir $Unit.Prefix)
}

# 暂存某个单元涉及的文件。
# 根单元**不能**用 git add -A：那会把还没同步的 8 个服务目录一起塞进根仓库的提交里，
# 所以根单元用 pathspec 排除法，只暂存根级内容。
function Get-UnitPathspec {
    param($Unit)
    if ($Unit.IsRoot) {
        return @('.') + ($serviceNames | ForEach-Object { ":(exclude)$_" })
    }
    return @($Unit.Prefix)
}

function Add-UnitToIndex {
    param($Unit)
    $paths = Get-UnitPathspec -Unit $Unit
    & git -C $PublishDir add -A -- @paths 2>&1 | Where-Object { $_ -notmatch 'will be replaced by' } | Out-Null
    return $LASTEXITCODE
}

# 该单元是否真的暂存出了改动
function Test-UnitStaged {
    param($Unit)
    $paths = Get-UnitPathspec -Unit $Unit
    & git -C $PublishDir diff --cached --quiet -- @paths 2>&1 | Out-Null
    return ($LASTEXITCODE -ne 0)
}

# 在聚合仓库里提交。传入原始时间时会**同时**把作者日期与提交者日期设为它，
# 这样 git log 的排序与显示就是源仓库的真实时间。
function Invoke-AggregateCommit {
    param(
        [string] $MessageFile,
        [string] $AuthorName,
        [string] $AuthorEmail,
        [string] $DateIso
    )
    $setDate = $false
    if ($DateIso) {
        $env:GIT_AUTHOR_DATE = $DateIso
        $env:GIT_COMMITTER_DATE = $DateIso
        $setDate = $true
    }
    try {
        if ($AuthorName -and $AuthorEmail) {
            & git -C $PublishDir commit "--author=$AuthorName <$AuthorEmail>" -F $MessageFile 2>&1 | ForEach-Object { Write-Host "    $_" }
        }
        else {
            & git -C $PublishDir commit -F $MessageFile 2>&1 | ForEach-Object { Write-Host "    $_" }
        }
        return $LASTEXITCODE
    }
    finally {
        # 必须清掉：否则后面的提交会继承上一条的日期
        if ($setDate) {
            Remove-Item Env:GIT_AUTHOR_DATE -ErrorAction SilentlyContinue
            Remove-Item Env:GIT_COMMITTER_DATE -ErrorAction SilentlyContinue
        }
    }
}

# 写提交信息文件并返回路径
function New-CommitMessageFile {
    param([string[]] $Lines)
    $f = Join-Path $env:TEMP ('publish-all-msg-' + [guid]::NewGuid().ToString('N') + '.txt')
    Write-Utf8NoBom -Path $f -Text (($Lines -join "`n") + "`n")
    return $f
}

# ===========================================================================
# 阶段 1：解析上次快照锚点
#   必须在 robocopy 覆盖 AGGREGATE.md **之前**做（根单元的 /XF 会保住这个文件，
#   但为了语义清晰，这里一次性在开头读完）
# ===========================================================================
function Read-PublishAnchors {
    param([string] $File)
    $map = @{}
    if (-not (Test-Path $File)) { return $map }
    $inBlock = $false
    foreach ($line in [System.IO.File]::ReadAllLines($File, [System.Text.UTF8Encoding]::new($false))) {
        if ($line -match '^\s*<!--\s*PUBLISH-ANCHORS') { $inBlock = $true; continue }
        if ($inBlock -and $line -match '^\s*-->') { $inBlock = $false; continue }
        if ($inBlock) {
            if ($line -match '^\s*(?<k>[A-Za-z0-9_\-]+)\s*=\s*(?<sha>[0-9a-fA-F]{7,40})\s*$') {
                $map[$Matches['k'].ToLowerInvariant()] = $Matches['sha']
            }
            continue
        }
        # 兼容旧版格式：从「快照时的各仓库版本」表格里取短 sha
        if ($line -match '^\|\s*(?<name>[^|]+?)\s*\|[^|]*\|\s*`(?<sha>[0-9a-fA-F]{7,40})`') {
            $n = $Matches['name'].Trim()
            $key = if ($n -like '*工作区根*') { 'workspace-root' } else { $n.ToLowerInvariant() }
            if (-not $map.ContainsKey($key)) { $map[$key] = $Matches['sha'] }
        }
    }
    return $map
}

Write-Host '--- 1/5 解析上次快照锚点 ---' -ForegroundColor Cyan
$anchors = Read-PublishAnchors -File (Join-Path $PublishDir 'AGGREGATE.md')
if ($ReplayAll -or $Fresh) {
    if ($ReplayAll) { Write-Host '  -ReplayAll：忽略锚点，重放全部历史' -ForegroundColor Yellow }
    if ($Fresh) { Write-Host '  -Fresh：忽略已有锚点' -ForegroundColor Yellow }
    $anchors = @{}
}
elseif ($anchors.Count -eq 0) {
    Write-Host '  未找到 AGGREGATE.md 或其中没有可用锚点' -ForegroundColor Yellow
    Write-Host '  → 各仓库本轮按「整体快照」提交一次（不会重放全部历史，避免内容重复）' -ForegroundColor Yellow
}
else {
    Write-Host ("  从 AGGREGATE.md 读到 $($anchors.Count) 个锚点")
}

# 每个单元解析出锚点对应的 commit，并标记本轮的处理方式
$unitPlan = New-Object System.Collections.Generic.List[object]
foreach ($u in $units) {
    $head = Get-GitScalar -Dir $u.Dir -GitArgs @('rev-parse', 'HEAD')
    if (-not $head) {
        Write-Host ('  ' + $u.Name.PadRight(22) + ' 不是 git 仓库或无提交，跳过') -ForegroundColor Yellow
        continue
    }
    $mode = 'batch'
    $base = $null
    if ($ReplayAll) {
        $mode = 'replay'
    }
    elseif ($anchors.ContainsKey($u.Key)) {
        $resolved = Get-GitScalar -Dir $u.Dir -GitArgs @('rev-parse', '--verify', '--quiet', "$($anchors[$u.Key])^{commit}")
        if ($resolved) { $mode = 'replay'; $base = $resolved }
        else { Write-Host ('  ' + $u.Name.PadRight(22) + " 锚点 $($anchors[$u.Key]) 在本仓库解析不到，退化为整体快照") -ForegroundColor Yellow }
    }
    $unitPlan.Add([pscustomobject]@{
            Key    = $u.Key
            Name   = $u.Name
            Dir    = $u.Dir
            Prefix = $u.Prefix
            IsRoot = $u.IsRoot
            Dest   = (Get-UnitDest $u)
            Head   = $head
            Mode   = $mode
            Base   = $base
        })
}

# ===========================================================================
# 阶段 2：规划待重放的提交
#   用 %x1f (Unit Separator) 作字段分隔符，避免和中文 / 逗号 / 竖线冲突
# ===========================================================================
Write-Host '--- 2/5 规划待重放提交 ---' -ForegroundColor Cyan
$replayList = New-Object System.Collections.Generic.List[object]
$SEP = [char]0x1f

foreach ($p in $unitPlan) {
    if ($p.Mode -ne 'replay') { continue }
    $range = if ($ReplayAll) { @('HEAD') } else { @("$($p.Base)..HEAD") }
    $raw = Invoke-GitLines -Dir $p.Dir -GitArgs (@('log', '--reverse', '--format=%H%x1f%aI%x1f%an%x1f%ae%x1f%s') + $range)
    foreach ($line in $raw) {
        if (-not $line) { continue }
        $f = "$line".Split($SEP)
        if ($f.Count -lt 5) { continue }
        $replayList.Add([pscustomobject]@{
                Key      = $p.Key
                Name     = $p.Name
                Dir      = $p.Dir
                Prefix   = $p.Prefix
                IsRoot   = $p.IsRoot
                Dest     = $p.Dest
                Sha      = $f[0]
                DateIso  = $f[1]
                DateObj  = ([datetimeoffset]::Parse($f[1]))
                Author   = $f[2]
                Email    = $f[3]
                Subject  = $f[4]
                ShortSha = $f[0].Substring(0, 8)
            })
    }
}

# 按作者时间排序 → 还原跨仓库的真实时间线交错
$replayList = @($replayList | Sort-Object -Property DateObj)

if ($replayList.Count -gt 0) {
    Write-Host ("  共 $($replayList.Count) 个提交待重放：")
    $i = 0
    foreach ($c in $replayList) {
        $i++
        Write-Host ('    {0,3}. [{1}] {2}  @{3}' -f $i, $c.DateObj.ToString('MM-dd HH:mm'), $c.Subject, $c.Key)
    }
}
else {
    Write-Host '  没有新提交需要重放'
}
Write-Host ''

$batchUnits = @($unitPlan | Where-Object { $_.Mode -eq 'batch' })
if ($batchUnits.Count -gt 0) {
    Write-Host '  以下单元无可用锚点，本轮按整体快照提交一次：' -ForegroundColor Yellow
    foreach ($b in $batchUnits) { Write-Host ('    - ' + $b.Name) -ForegroundColor Yellow }
    Write-Host ''
}

if ($DryRun) {
    Write-Host 'DryRun 结束：未做任何本地/远端改动。' -ForegroundColor Yellow
    exit 0
}

# ===========================================================================
# 阶段 3：逐条重放
#   每个单元复用一个 git worktree（--detach 到目标 revision），避免反复解包，
#   也避免依赖 tar / zip 解包带来的中文文件名编码风险（git 自己 checkout 最可靠）
# ===========================================================================
Write-Host '--- 3/5 逐条重放提交 ---' -ForegroundColor Cyan
if (-not (Test-Path $PublishDir)) { New-Item -ItemType Directory -Path $PublishDir -Force | Out-Null }

Push-Location $PublishDir
try {
    if (-not (Test-Path '.git')) {
        git init -b main 2>&1 | Out-Null
        Write-Host '  已初始化聚合仓库'
    }

    $remoteUrl = "https://github.com/$Owner/$RepoName.git"
    if ((git remote) -contains 'origin') { git remote set-url origin $remoteUrl }
    else { git remote add origin $remoteUrl }
}
finally {
    Pop-Location
}

$workRoot = Join-Path $env:TEMP ('publish-all-wt-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $workRoot -Force | Out-Null
$worktrees = @{}

try {
    if ($replayList.Count -eq 0) { Write-Host '  跳过' }

    $idx = 0
    foreach ($c in $replayList) {
        $idx++

        # --- 取出该 revision 的完整树
        if (-not $worktrees.ContainsKey($c.Key)) {
            $wt = Join-Path $workRoot $c.Key
            & git -C $c.Dir worktree prune 2>&1 | Out-Null
            & git -C $c.Dir worktree add --detach $wt $c.Sha 2>&1 | Out-Null
            if ($LASTEXITCODE -ne 0) { throw "git worktree add 失败: $($c.Dir) @ $($c.Sha)" }
            $worktrees[$c.Key] = $wt
        }
        $wt = $worktrees[$c.Key]
        & git -C $wt checkout --detach --quiet $c.Sha 2>&1 | Out-Null

        # --- 镜像到聚合仓库的对应前缀
        $rc = Sync-UnitTree -From $wt -To $c.Dest -IsRoot $c.IsRoot
        if ($rc -ge 8) { throw "robocopy 失败（退出码 $rc）: $wt -> $($c.Dest)" }

        # --- 暂存并提交
        Add-UnitToIndex -Unit $c | Out-Null
        if (-not (Test-UnitStaged -Unit $c)) {
            Write-Host ('  {0,3}/{1} [跳过·无内容变化] {2}' -f $idx, $replayList.Count, $c.Subject) -ForegroundColor DarkGray
            continue
        }

        # 正文取原文（%B 含主题 + 正文），末尾追加来源标注
        $rawBody = (Invoke-GitLines -Dir $c.Dir -GitArgs @('show', '-s', '--format=%B', $c.Sha)) -join "`n"
        $bodyLines = New-Object System.Collections.Generic.List[string]
        foreach ($l in ($rawBody -split "`r?`n")) { $bodyLines.Add($l.TrimEnd()) }
        while ($bodyLines.Count -gt 0 -and [string]::IsNullOrWhiteSpace($bodyLines[$bodyLines.Count - 1])) {
            $bodyLines.RemoveAt($bodyLines.Count - 1)
        }
        $bodyLines.Add('')
        $bodyLines.Add("Source-Repo: $($c.Key)")
        $bodyLines.Add("Source-Commit: $($c.ShortSha)")

        $msgFile = New-CommitMessageFile -Lines $bodyLines.ToArray()
        $rc = Invoke-AggregateCommit -MessageFile $msgFile -AuthorName $c.Author -AuthorEmail $c.Email -DateIso $c.DateIso
        Remove-Item $msgFile -Force -ErrorAction SilentlyContinue
        if ($rc -ne 0) { throw "提交失败: $($c.Sha) $($c.Subject)" }

        Write-Host ('  {0,3}/{1} [{2}] {3}  @{4}' -f $idx, $replayList.Count, $c.DateObj.ToString('MM-dd HH:mm'), $c.Subject, $c.Key)
    }

    # ---------------------------------------------------------------- 无锚点单元：整体快照
    foreach ($b in $batchUnits) {
        $rc = Sync-UnitTree -From $b.Dir -To $b.Dest -IsRoot $b.IsRoot
        if ($rc -ge 8) { throw "robocopy 失败（退出码 $rc）: $($b.Dir) -> $($b.Dest)" }
        Add-UnitToIndex -Unit $b | Out-Null
        if (-not (Test-UnitStaged -Unit $b)) { continue }

        $headInfo = ((Invoke-GitLines -Dir $b.Dir -GitArgs @('log', '-1', '--format=%h %s')) -join '')
        $msgFile = New-CommitMessageFile -Lines @(
            "snapshot($($b.Key)): 纳入聚合快照",
            '',
            "- 当前 HEAD: $headInfo",
            '- 说明: 上次快照未记录本仓库锚点，本轮按整体快照提交，后续提交将逐条重放。'
        )
        $rc = Invoke-AggregateCommit -MessageFile $msgFile
        Remove-Item $msgFile -Force -ErrorAction SilentlyContinue
        if ($rc -ne 0) { throw "提交失败: $($b.Key) 整体快照" }
        Write-Host ('  [整体快照] ' + $b.Name) -ForegroundColor Yellow
    }

    # ---------------------------------------------------------------- 阶段 4：未提交改动
    Write-Host '--- 4/5 同步未提交的工作树改动 ---' -ForegroundColor Cyan
    $dirtyCount = 0
    foreach ($u in $units) {
        if (-not (Test-Path $u.Dir)) { continue }
        $status = Invoke-GitLines -Dir $u.Dir -GitArgs @('status', '--porcelain')
        if ($status.Count -eq 0) { continue }

        $dest = Get-UnitDest $u
        $rc = Sync-UnitTree -From $u.Dir -To $dest -IsRoot $u.IsRoot
        if ($rc -ge 8) { throw "robocopy 失败（退出码 $rc）: $($u.Dir) -> $dest" }

        $ref = $unitPlan | Where-Object { $_.Key -eq $u.Key } | Select-Object -First 1
        if (-not $ref) { $ref = $u }
        Add-UnitToIndex -Unit $ref | Out-Null
        if (-not (Test-UnitStaged -Unit $ref)) { continue }

        $fileCount = @($status | Where-Object { $_ }).Count
        $msgFile = New-CommitMessageFile -Lines @(
            "snapshot($($u.Key)): 未提交改动快照",
            '',
            "- 涉及 $fileCount 个文件（git status 条目数）",
            '- 说明: 这部分在源仓库里尚未提交，属临时快照；源仓库提交后会以真实提交信息重放。'
        )
        $rc = Invoke-AggregateCommit -MessageFile $msgFile
        Remove-Item $msgFile -Force -ErrorAction SilentlyContinue
        if ($rc -ne 0) { throw "提交失败: $($u.Key) 未提交改动" }
        $dirtyCount++
        Write-Host ('  [未提交改动] ' + $u.Name + "  ($fileCount 个文件)")
    }
    if ($dirtyCount -eq 0) { Write-Host '  所有仓库工作树干净' }
}
finally {
    # 清理临时 worktree（失败也要清，否则源仓库的 .git/worktrees 会留垃圾）
    foreach ($k in $worktrees.Keys) {
        $owner = $units | Where-Object { $_.Key -eq $k } | Select-Object -First 1
        if ($owner) {
            & git -C $owner.Dir worktree remove --force $worktrees[$k] 2>&1 | Out-Null
            & git -C $owner.Dir worktree prune 2>&1 | Out-Null
        }
    }
    Remove-Item $workRoot -Recurse -Force -ErrorAction SilentlyContinue
}

# ===========================================================================
# 阶段 5：刷新快照元数据（.gitignore / .gitattributes / AGGREGATE.md）并推送
# ===========================================================================
Write-Host '--- 5/5 刷新快照元数据并推送 ---' -ForegroundColor Cyan
Push-Location $PublishDir
try {
    # 聚合专用 .gitignore：与工作区根仓库的不同 —— 根仓库刻意排除 8 个服务目录
    # （它们各自是独立仓库），而聚合仓库必须收录它们。
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
    Write-Utf8NoBom -Path (Join-Path $PublishDir '.gitignore') -Text $gitignore

    # 统一换行符为 LF。本机 core.autocrlf=true 时，若不声明，每次 add 都会对几百个
    # 文件刷一行 "LF will be replaced by CRLF" 警告，噪音很大且没有意义。
    $gitattributes = @'
# 本文件由 tools/publish-all.ps1 自动生成，请勿手工编辑
# 统一以 LF 入库并在检出时保持 LF，避免 Windows 上 core.autocrlf 带来的换行符抖动。
* text=auto eol=lf
'@
    Write-Utf8NoBom -Path (Join-Path $PublishDir '.gitattributes') -Text $gitattributes

    # 收集「快照后的」各仓库版本，作为下一轮的锚点
    function Get-RepoVersion {
        param([string] $Dir)
        if (-not (Test-Path (Join-Path $Dir '.git'))) { return $null }
        $sha = Get-GitScalar -Dir $Dir -GitArgs @('rev-parse', 'HEAD')
        if (-not $sha) { return $null }
        return [pscustomobject]@{
            Full   = $sha
            Commit = $sha.Substring(0, 7)
            Branch = (Get-GitScalar -Dir $Dir -GitArgs @('rev-parse', '--abbrev-ref', 'HEAD'))
            Tags   = ((Invoke-GitLines -Dir $Dir -GitArgs @('tag', '--points-at', 'HEAD')) -join ', ')
            Dirty  = ((Invoke-GitLines -Dir $Dir -GitArgs @('status', '--porcelain')).Count -gt 0)
        }
    }

    $now = Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz'
    $versions = [ordered]@{}
    foreach ($u in $units) { $versions[$u.Key] = Get-RepoVersion $u.Dir }

    # 注意：下面全部用单引号字符串拼接。
    # 双引号字符串里 ` 是转义符，`$ 会变成字面量 $，`" 会变成字面量引号并导致字符串不闭合，
    # 而我们要输出的恰恰是 markdown 的反引号，所以只能用单引号（单引号内 ` 是普通字符）。
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add('# 聚合总仓库说明')
    $lines.Add('')
    $lines.Add('本仓库由 `tools/publish-all.ps1` **自动生成**，是 9 个独立本地仓库的一份只读快照，')
    $lines.Add('目的是「远端只有一个总仓库、方便浏览与分享」，**不是开发仓库**。')
    $lines.Add('')
    $lines.Add('> 开发与完整提交历史请使用本地各自的仓库：')
    $lines.Add('> 8 个服务仓库 + 1 个工作区根仓库（infra：`deploy/`、`tools/`、跨服务文档）。')
    $lines.Add('')
    $lines.Add('快照时间：`' + $now + '`')
    $lines.Add('')
    $lines.Add('## 提交历史怎么来的')
    $lines.Add('')
    $lines.Add('本仓库的提交**逐条对应各源仓库的真实提交**（主题 / 正文 / 作者 / 提交时间均保留），')
    $lines.Add('仅在正文末尾追加两行来源标注：')
    $lines.Add('')
    $lines.Add('```text')
    $lines.Add('Source-Repo: gateway-service')
    $lines.Add('Source-Commit: afd0344')
    $lines.Add('```')
    $lines.Add('')
    $lines.Add('- 跨仓库按**作者时间**交错排列，`git log` 看到的就是真实的时间线。')
    $lines.Add('- 脚本靠本文件里的锚点做增量，同一批提交只会被重放一次。')
    $lines.Add('- `snapshot(<仓库>): 未提交改动快照` 表示对应内容在源仓库里**尚未提交**，属临时快照。')
    $lines.Add('')
    $lines.Add('## 快照时的各仓库版本')
    $lines.Add('')
    $lines.Add('| 仓库 | 分支 | commit | tag | 备注 |')
    $lines.Add('| --- | --- | --- | --- | --- |')
    foreach ($u in $units) {
        $v = $versions[$u.Key]
        if ($v) {
            $note = if ($v.Dirty) { '快照时存在未提交改动' } else { '' }
            $lines.Add('| ' + $u.Name + ' | ' + $v.Branch + ' | `' + $v.Commit + '` | ' + $v.Tags + ' | ' + $note + ' |')
        }
        else {
            $lines.Add('| ' + $u.Name + ' | - | - | - | 不是 git 仓库 |')
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
    $lines.Add('脚本会：按锚点算出各仓库的新提交 → 逐条重放到本仓库（保留原始提交信息）')
    $lines.Add('→ 同步未提交改动 → 刷新本文件 → 推送。')
    $lines.Add('')
    $lines.Add('## 目录构成')
    $lines.Add('')
    $lines.Add('- `deploy/` —— 部署编排（compose / 环境变量模板 / 数据库基线 / K8s / SRS 配置）')
    $lines.Add('- 8 个服务目录 —— `auth` / `gateway` / `user` / `product` / `order`（Java）、`video` / `live` / `danmaku`（Go）')
    $lines.Add('- 根目录 `*.md` —— 跨服务设计文档、接口测试文档、公共约定与问题记录')
    $lines.Add('')
    $lines.Add('## 机器可读锚点')
    $lines.Add('')
    $lines.Add('下面这段由脚本读取与写入，是增量重放的判断依据，**请勿手工修改**。')
    $lines.Add('')
    $lines.Add('<!-- PUBLISH-ANCHORS')
    foreach ($u in $units) {
        $v = $versions[$u.Key]
        if ($v) { $lines.Add(($u.Key + ' = ' + $v.Full)) }
    }
    $lines.Add('-->')
    Write-Utf8NoBom -Path (Join-Path $PublishDir 'AGGREGATE.md') -Text (($lines -join "`n") + "`n")

    # 元数据提交
    $metaPaths = @('.gitignore', '.gitattributes', 'AGGREGATE.md')
    & git -C $PublishDir add -A -- @metaPaths 2>&1 |
        Where-Object { $_ -notmatch 'will be replaced by' } | Out-Null
    & git -C $PublishDir diff --cached --quiet -- @metaPaths 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        if (-not $Message) { $Message = "chore: 刷新聚合快照元数据 $now" }
        $body = New-Object System.Collections.Generic.List[string]
        $body.Add($Message)
        $body.Add('')
        $body.Add('各仓库快照版本：')
        foreach ($u in $units) {
            if ($versions[$u.Key]) { $body.Add("- $($u.Key) = $($versions[$u.Key].Commit)") }
        }
        $msgFile = New-CommitMessageFile -Lines $body.ToArray()
        & git -C $PublishDir commit -F $msgFile 2>&1 | ForEach-Object { Write-Host "  $_" }
        Remove-Item $msgFile -Force -ErrorAction SilentlyContinue
    }
    else {
        Write-Host '  元数据无变化，跳过提交'
    }

    $leftover = @(git status --porcelain)
    if ($leftover.Count -eq 0) {
        Write-Host '  聚合目录工作树干净'
    }
    else {
        Write-Host '  注意：聚合目录仍有未提交改动：' -ForegroundColor Yellow
        $leftover | Select-Object -First 20 | ForEach-Object { Write-Host "    $_" -ForegroundColor Yellow }
    }

    # 远端不存在则创建
    $env:Path = [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [Environment]::GetEnvironmentVariable('Path', 'User')
    & gh repo view "$Owner/$RepoName" --json name 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Write-Host '  远端总仓库不存在，创建中 ...'
        & gh repo create "$Owner/$RepoName" "--$Visibility" 2>&1 | ForEach-Object { Write-Host "    $_" }
        # 探测可能瞬时失败，再确认一次
        & gh repo view "$Owner/$RepoName" --json name 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "创建远端仓库失败: $Owner/$RepoName" }
    }

    $branch = (git rev-parse --abbrev-ref HEAD).Trim()
    if ($ReplayAll) {
        Write-Host '  -ReplayAll 重建了历史，使用 force push' -ForegroundColor Yellow
        git push -u origin $branch --force 2>&1 | ForEach-Object { Write-Host "  $_" }
    }
    else {
        git push -u origin $branch 2>&1 | ForEach-Object { Write-Host "  $_" }
    }
    if ($LASTEXITCODE -ne 0) { throw 'git push 失败' }

    Write-Host ''
    Write-Host '--- 聚合仓库最近提交 ---' -ForegroundColor Cyan
    git log --oneline -10
}
finally {
    Pop-Location
}

Write-Host ''
Write-Host "完成：https://github.com/$Owner/$RepoName" -ForegroundColor Green

# 显式退出码：PowerShell 5.1 下 -File 的退出码会受前面原生命令的 $LASTEXITCODE 影响
# （例如 gh repo view 探测「不存在」时会留下 1），所以这里明确收口。
exit 0
