#Requires -Version 5.1
<#
.SYNOPSIS
    把本地 9 个仓库（8 个服务 + 根 infra 仓库）推送到 GitHub。

.DESCRIPTION
    本地结构是「每个服务一个独立 git 仓库」（9 个），GitHub 上也一一对应建 9 个远端仓库。
    本脚本可重复运行：
      - 远端不存在 → gh repo create 建仓并推送
      - 远端已存在 → 直接 git push（用于日常更新）

    为什么不把 8 个服务塞进一个远端仓库：
    git 遇到含 .git 的子目录只会记一个 gitlink（commit 指针），clone 下来是 8 个空目录。
    要让一个仓库装下全部内容，只能另建镜像仓库用 robocopy 同步（多一份维护成本）。

.PARAMETER Owner
    GitHub 账号名。默认取 git config 里 noreply 邮箱中的用户名。

.PARAMETER Visibility
    private（默认）或 public。注意：各服务 configs/config.yaml 与 application.yml 里有
    明文数据库口令，默认用 private，公开前请先确认。

.PARAMETER RootRepoName
    根 infra 仓库（deploy/ + 跨服务文档）的远端仓库名，默认 service-infra。

.PARAMETER Tag
    可选。若某仓库当前没有任何 tag，则用它创建并推送一个初始 tag（例如 v0.1.0）。

.PARAMETER DryRun
    只打印将要执行的动作，不建仓、不推送、不需要 gh 已登录。

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File tools/push-to-github.ps1 -DryRun

.EXAMPLE
    powershell -NoProfile -ExecutionPolicy Bypass -File tools/push-to-github.ps1 -Tag v0.1.0
#>
[CmdletBinding()]
param(
    [string] $Owner,
    [ValidateSet('private', 'public')] [string] $Visibility = 'private',
    [string] $RootRepoName = 'service-infra',
    [string] $Tag,
    [switch] $DryRun
)

# 注意：不能用 'Stop'。PowerShell 5.1 会把原生命令（gh / git）写到 stderr 的内容包成
# ErrorRecord，配合 'Stop' 会直接把脚本终止；而 `gh repo view` 在判断「仓库不存在」时
# 本来就会往 stderr 写一行 GraphQL 错误（这属于预期分支，不是真的失败）。
# 所以统一用 'Continue'，并在每个原生命令后显式检查 $LASTEXITCODE。
$ErrorActionPreference = 'Continue'

# ---------------------------------------------------------------- 定位工作区根
$root = Split-Path $PSScriptRoot -Parent

function Test-GhLoggedIn {
    gh auth status 2>&1 | Out-Null
    return ($LASTEXITCODE -eq 0)
}

function Test-GhRepoExists {
    param([string] $FullName)
    gh repo view $FullName --json name 2>&1 | Out-Null
    return ($LASTEXITCODE -eq 0)
}

# ---------------------------------------------------------------- 推断 Owner
if (-not $Owner) {
    $email = git config --global user.email
    if ($email -match '^\d+\+(.+)@users\.noreply\.github\.com$') {
        $Owner = $Matches[1]
    }
}
if (-not $Owner) {
    throw '无法从 git config 推断 GitHub 用户名，请显式传 -Owner'
}

# ---------------------------------------------------------------- 待推送的 9 个仓库
$targets = @(
    @{ Path = Join-Path $root 'auth-service';       Name = 'auth-service' }
    @{ Path = Join-Path $root 'gateway-service';    Name = 'gateway-service' }
    @{ Path = Join-Path $root 'user-service';       Name = 'user-service' }
    @{ Path = Join-Path $root 'product-service';    Name = 'product-service' }
    @{ Path = Join-Path $root 'order-service';      Name = 'order-service' }
    @{ Path = Join-Path $root 'video-service-go';   Name = 'video-service-go' }
    @{ Path = Join-Path $root 'live-service-go';    Name = 'live-service-go' }
    @{ Path = Join-Path $root 'danmaku-service-go'; Name = 'danmaku-service-go' }
    @{ Path = $root;                                Name = $RootRepoName }
)

Write-Host "GitHub 账号 : $Owner" -ForegroundColor Cyan
Write-Host "可见性      : $Visibility" -ForegroundColor Cyan
Write-Host "仓库数量    : $($targets.Count)" -ForegroundColor Cyan
if ($DryRun) { Write-Host "模式        : DryRun（不产生任何远端改动）" -ForegroundColor Yellow }
Write-Host ''

# ---------------------------------------------------------------- 前置：gh 已登录
if (-not $DryRun) {
    if (-not (Test-GhLoggedIn)) {
        throw 'gh 未登录。请先执行： gh auth login --hostname github.com --git-protocol https --web'
    }
}

$ok = 0
$failed = @()

foreach ($t in $targets) {
    Write-Host "===== $($t.Name) =====" -ForegroundColor Cyan
    Push-Location $t.Path
    try {
        # 1) 工作区必须干净：避免把半成品推上去（有改动就跳过，让人先处理）
        $dirty = git status --porcelain
        if ($dirty) {
            Write-Host '  跳过：存在未提交改动' -ForegroundColor Yellow
            $dirty | ForEach-Object { Write-Host "    $_" }
            $failed += "$($t.Name)（有未提交改动）"
            continue
        }

        $branch = (git rev-parse --abbrev-ref HEAD).Trim()
        $remoteUrl = "https://github.com/$Owner/$($t.Name).git"

        if ($DryRun) {
            Write-Host "  将创建 https://github.com/$Owner/$($t.Name) 并推送分支 $branch"
            $tags = git tag
            Write-Host ("  现有 tag： " + $(if ($tags) { ($tags -join ', ') } else { '（无）' }))
            $ok++
            continue
        }

        # 2) 远端不存在则创建（--source 会自动加上 origin 并推送）
        $exists = Test-GhRepoExists "$Owner/$($t.Name)"

        if (-not $exists) {
            Write-Host "  创建远端并推送 ..."
            gh repo create "$Owner/$($t.Name)" "--$Visibility" --source . --remote origin --push
            if ($LASTEXITCODE -ne 0) {
                Write-Host '  失败：gh repo create' -ForegroundColor Red
                $failed += "$($t.Name)（创建失败）"
                continue
            }
        }
        else {
            Write-Host "  远端已存在，直接推送 ..."
            if (-not ((git remote) -contains 'origin')) { git remote add origin $remoteUrl }
            git push -u origin $branch
            if ($LASTEXITCODE -ne 0) {
                Write-Host '  失败：git push' -ForegroundColor Red
                $failed += "$($t.Name)（推送失败）"
                continue
            }
        }

        # 3) 初始 tag（可选）：让「可用版本」在远端有标记
        if ($Tag -and -not (git tag)) {
            git tag -a $Tag -m "$($t.Name) $Tag"
            git push origin $Tag
            Write-Host "  已打 tag：$Tag"
        }

        # 4) 已有的 tag 一并推送
        if (git tag) { git push origin --tags 2>&1 | Out-Null }

        Write-Host "  OK  https://github.com/$Owner/$($t.Name)" -ForegroundColor Green
        $ok++
    }
    finally {
        Pop-Location
    }
}

Write-Host ''
Write-Host "========== 完成：成功 $ok / $($targets.Count) ==========" -ForegroundColor Cyan
if ($failed) {
    Write-Host '以下仓库未处理：' -ForegroundColor Yellow
    $failed | ForEach-Object { Write-Host "  - $_" }
    exit 1
}
