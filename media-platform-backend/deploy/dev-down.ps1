# 停止 dev-up.ps1 启动的 8 个服务
#
# 用法：
#   .\dev-down.ps1              # 停服务 + 停基础设施
#   .\dev-down.ps1 -KeepInfra   # 只停服务，保留 MySQL/Redis/MinIO/Nacos/SRS

[CmdletBinding()]
param([switch]$KeepInfra)

$ErrorActionPreference = 'Continue'
$deployDir = $PSScriptRoot
$runtimeDir = Join-Path $deployDir 'runtime'
$pidFile = Join-Path $runtimeDir 'pids.txt'

if (Test-Path $pidFile) {
    foreach ($line in Get-Content $pidFile -Encoding ASCII) {
        if ($line -match '^([^=]+)=(\d+)$') {
            $name = $Matches[1]
            $processId = [int]$Matches[2]
            Write-Host "[stop] $name (pid=$processId)"
            # /T 连同 cmd 的子进程（java / go）一起结束
            taskkill /PID $processId /T /F *> $null
        }
    }
    Remove-Item $pidFile -Force
}

# 兜底：按端口回收（防止手工启动或 pid 文件丢失）
foreach ($port in 8000, 8001, 8002, 8003, 8004, 9001, 9002, 9003, 20880) {
    $conn = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($conn) {
        Write-Host "[stop] 端口 $port (pid=$($conn.OwningProcess))"
        taskkill /PID $conn.OwningProcess /T /F *> $null
    }
}

if (-not $KeepInfra) {
    Write-Host '[infra] 停止基础设施 ...'
    Push-Location $deployDir
    # 必须显式带上 profile：docker compose down 默认只处理未挂 profile 的服务，
    # 不加 --profile infra 时 MySQL/Redis/MinIO/Nacos/SRS 会继续跑（实测踩过）。
    try { docker compose --profile infra --profile media --profile java --profile web down | Out-Null } finally { Pop-Location }
}
else {
    Write-Host '[infra] 保留基础设施（-KeepInfra）'
}
