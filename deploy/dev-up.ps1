# 一键启动本地全栈（基础设施 + 8 个服务）
#
# 用法：
#   .\dev-up.ps1                 # 起 infra（若未起）+ 导表 + 启动 8 个服务
#   .\dev-up.ps1 -SkipInfra      # 只启动服务（infra 已在跑）
#   .\dev-up.ps1 -SkipBuild      # 跳过 Go/Java 构建，直接用已有产物
#
# 设计说明：
#   - .env 必须用 -Encoding UTF8 读取（PowerShell 5.1 默认 ANSI 会吞换行，见 E-005）
#   - MySQL 口令以容器实际值为准（.env 可能被 Compose 插值，见 I-018）
#   - 服务进程通过 runtime/*.cmd 启动，PID 记录在 runtime/pids.txt，dev-down.ps1 负责回收

[CmdletBinding()]
param(
    [switch]$SkipInfra,
    [switch]$SkipBuild
)

# 原生命令（docker/mvn/go）的 stderr 在 PS 5.1 下会被当成错误记录，
# 用 Continue 并在需要的地方显式检查 $LASTEXITCODE。
$ErrorActionPreference = 'Continue'
$deployDir = $PSScriptRoot
$backendRoot = Split-Path -Parent $deployDir
$repoRoot = Split-Path -Parent $backendRoot
$runtimeDir = Join-Path $deployDir 'runtime'
$envFile = Join-Path $deployDir '.env'

if (-not (Test-Path $envFile)) {
    throw "缺少 $envFile，请先复制 .env.example 并填写"
}

# ---------------------------------------------------------------- 读取 .env（UTF-8！）
$cfg = @{}
foreach ($line in Get-Content $envFile -Encoding UTF8) {
    if ($line -match '^([A-Z_]+)=(.*)$') {
        $cfg[$Matches[1]] = $Matches[2].Trim().Trim('"')
    }
}
if (-not $cfg['RABBITMQ_USER']) { throw '.env 解析失败：RABBITMQ_USER 为空（确认用 -Encoding UTF8 读取）' }

New-Item -ItemType Directory -Force -Path $runtimeDir | Out-Null
$pidFile = Join-Path $runtimeDir 'pids.txt'
Set-Content -Path $pidFile -Value '' -Encoding ASCII

# ---------------------------------------------------------------- 基础设施
if (-not $SkipInfra) {
    docker info 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Docker 未启动' }

    Write-Host '[infra] 启动 MySQL / Redis / RabbitMQ / MinIO / Nacos ...'
    Push-Location $deployDir
    try {
        docker compose --profile infra up -d mysql redis rabbitmq minio nacos | Out-Null
        # SRS 的 WebRTC 默认映射 8001，会和 user-service 冲突，这里固定改到 8100
        $env:SRS_WEBRTC_HOST_PORT = '8100'
        docker compose --profile media up -d srs | Out-Null
    }
    finally {
        Pop-Location
    }
}

# 等 MySQL ready（用容器自带 healthcheck，避免 mysqladmin 需要口令的问题）
Write-Host '[infra] 等待 MySQL 就绪 ...'
$deadline = (Get-Date).AddSeconds(180)
do {
    Start-Sleep -Seconds 3
    $mysqlHealth = (docker inspect --format '{{.State.Health.Status}}' gomedia-mysql 2>$null | Out-String).Trim()
} while ($mysqlHealth -ne 'healthy' -and (Get-Date) -lt $deadline)
if ($mysqlHealth -ne 'healthy') { throw "MySQL 180s 内未就绪（health=$mysqlHealth）" }

$dbUser = 'root'
$dbPw = (docker exec gomedia-mysql printenv MYSQL_ROOT_PASSWORD).Trim()
if (-not $dbPw) { throw '拿不到容器 MySQL 口令' }
$dsnJdbc = "jdbc:mysql://127.0.0.1:3307/auth_service?useUnicode=true&characterEncoding=utf8&serverTimezone=Asia/Shanghai&useSSL=false&allowPublicKeyRetrieval=true"
$dsnGo = "$dbUser`:$dbPw@tcp(127.0.0.1:3307)/auth_service?charset=utf8mb4&parseTime=True&loc=Local"

# ---------------------------------------------------------------- 建表 + 种子
function Invoke-MySqlFile([string]$localPath, [string]$remoteName) {
    docker cp $localPath "gomedia-mysql:/tmp/$remoteName" | Out-Null
    docker exec gomedia-mysql sh -c "MYSQL_PWD=`"`$MYSQL_ROOT_PASSWORD`" mysql -uroot --default-character-set=utf8mb4 auth_service < /tmp/$remoteName"
    if ($LASTEXITCODE -ne 0) { throw "导入 $localPath 失败" }
}

$schemaCount = (('SELECT COUNT(*) FROM information_schema.tables WHERE table_schema="auth_service" AND table_name="product";' |
    docker exec -i gomedia-mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -uroot -N' 2>$null) -join '').Trim()
if ($schemaCount -eq '0') {
    Write-Host '[db] 导入 product / order schema ...'
    Invoke-MySqlFile (Join-Path $backendRoot 'product-service\src\main\resources\db\schema.sql') 'product.sql'
    Invoke-MySqlFile (Join-Path $backendRoot 'order-service\src\main\resources\db\schema.sql') 'order.sql'
}

Write-Host '[db] 写入演示种子数据（幂等）...'
Invoke-MySqlFile (Join-Path $deployDir 'db\seed-media.sql') 'seed-media.sql'

# ---------------------------------------------------------------- MinIO 桶
Write-Host '[minio] 建桶 + 匿名只读策略 ...'
$minioEnv = @{
    VIDEO_MINIO_ENDPOINT   = '127.0.0.1:9000'
    VIDEO_MINIO_ACCESS_KEY = $cfg['MINIO_ROOT_USER']
    VIDEO_MINIO_SECRET_KEY = $cfg['MINIO_ROOT_PASSWORD']
    VIDEO_MINIO_BUCKET     = $cfg['MEDIA_BUCKET']
    GOCACHE                = (Join-Path $repoRoot '.gocache')
}
$oldEnv = @{}
foreach ($k in $minioEnv.Keys) { $oldEnv[$k] = [Environment]::GetEnvironmentVariable($k); [Environment]::SetEnvironmentVariable($k, $minioEnv[$k]) }
Push-Location (Join-Path $backendRoot 'video-service-go')
try { go run ./cmd/bootstrap-minio | Out-Host } finally { Pop-Location }
foreach ($k in $oldEnv.Keys) { [Environment]::SetEnvironmentVariable($k, $oldEnv[$k]) }

# ---------------------------------------------------------------- 构建
if (-not $SkipBuild) {
    Write-Host '[build] Go 服务 ...'
    $goServices = @('video-service-go', 'live-service-go', 'danmaku-service-go')
    foreach ($svc in $goServices) {
        Push-Location (Join-Path $backendRoot $svc)
        try {
            $env:GOCACHE = Join-Path $repoRoot '.gocache'
            go build -o (Join-Path $runtimeDir "$svc.exe") ./cmd/server
            if ($LASTEXITCODE -ne 0) { throw "$svc 构建失败" }
        }
        finally { Pop-Location }
    }

    Write-Host '[build] Java 服务（仅在缺 jar 时打包）...'
    $javaServices = @(
        @{ dir = 'user-service';    jar = 'user-service-0.0.1-SNAPSHOT.jar' },
        @{ dir = 'auth-service';    jar = 'auth-service-0.0.1-SNAPSHOT.jar' },
        @{ dir = 'product-service'; jar = 'product-service-1.0.0.jar' },
        @{ dir = 'order-service';   jar = 'order-service-1.0.0.jar' },
        @{ dir = 'gateway-service'; jar = 'gateway-service-0.0.1-SNAPSHOT.jar' }
    )
    foreach ($svc in $javaServices) {
        $jarPath = Join-Path $backendRoot "$($svc.dir)\target\$($svc.jar)"
        if (-not (Test-Path $jarPath)) {
            Push-Location (Join-Path $backendRoot $svc.dir)
            try {
                mvn -q -DskipTests package
                if ($LASTEXITCODE -ne 0) { throw "$($svc.dir) 打包失败" }
            }
            finally { Pop-Location }
        }
    }
}

# ---------------------------------------------------------------- 启动服务
$jwtSecret = 'dev-only-jwt-secret-please-change-in-production-0123456789'
$ffmpeg = (Get-Command ffmpeg -ErrorAction SilentlyContinue).Source
$ffmpegPath = if ($ffmpeg) { $ffmpeg.Replace('\', '/') } else { '' }

function Start-ServiceProcess {
    param([string]$Name, [string]$WorkDir, [hashtable]$EnvMap, [string]$Exe, [string[]]$Arguments)

    # 不用 .cmd 包装：工作区路径含中文，而 cmd 按 OEM 代码页解析脚本，
    # 写成 ASCII 会把「作品集」变成 ???（实测启动失败）。直接给子进程设环境变量即可。
    $saved = @{}
    foreach ($key in $EnvMap.Keys) {
        $saved[$key] = [Environment]::GetEnvironmentVariable($key)
        [Environment]::SetEnvironmentVariable($key, $EnvMap[$key])
    }

    $stdout = Join-Path $runtimeDir "$Name.log"
    $stderr = Join-Path $runtimeDir "$Name.err.log"
    try {
        $process = Start-Process -FilePath $Exe `
            -ArgumentList $Arguments `
            -WorkingDirectory $WorkDir `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdout `
            -RedirectStandardError $stderr `
            -PassThru
    }
    finally {
        foreach ($key in $saved.Keys) {
            [Environment]::SetEnvironmentVariable($key, $saved[$key])
        }
    }
    Add-Content -Path $pidFile -Value "$Name=$($process.Id)" -Encoding ASCII
    Write-Host ("  {0,-16} pid={1}" -f $Name, $process.Id)
}

Write-Host '[run] 启动 Go 服务 ...'
Start-ServiceProcess -Name 'video-service' -WorkDir (Join-Path $backendRoot 'video-service-go') -EnvMap @{
    APP_ENV                    = 'local'
    VIDEO_MYSQL_DSN            = $dsnGo
    VIDEO_REDIS_ADDR           = '127.0.0.1:6380'
    VIDEO_RABBITMQ_URL         = "amqp://$($cfg['RABBITMQ_USER']):$($cfg['RABBITMQ_PASSWORD'])@127.0.0.1:5672/"
    VIDEO_MINIO_ENDPOINT       = '127.0.0.1:9000'
    VIDEO_MINIO_PUBLIC_ENDPOINT = 'http://127.0.0.1:9000'
    VIDEO_MINIO_ACCESS_KEY     = $cfg['MINIO_ROOT_USER']
    VIDEO_MINIO_SECRET_KEY     = $cfg['MINIO_ROOT_PASSWORD']
    VIDEO_MINIO_BUCKET         = $cfg['MEDIA_BUCKET']
    VIDEO_TRANSCODER_DRIVER    = if ($ffmpegPath) { 'real' } else { 'mock' }
    VIDEO_FFMPEG_PATH          = $ffmpegPath
} -Exe (Join-Path $runtimeDir 'video-service-go.exe') -Arguments @('-config','configs/config.yaml')

Start-ServiceProcess -Name 'live-service' -WorkDir (Join-Path $backendRoot 'live-service-go') -EnvMap @{
    APP_ENV              = 'local'
    LIVE_MYSQL_DSN       = $dsnGo
    LIVE_REDIS_ADDR      = '127.0.0.1:6380'
    LIVE_RABBITMQ_URL    = "amqp://$($cfg['RABBITMQ_USER']):$($cfg['RABBITMQ_PASSWORD'])@127.0.0.1:5672/"
    LIVE_MEDIA_DRIVER    = 'srs'
    LIVE_SRS_RTMP_HOST   = '127.0.0.1'
    LIVE_SRS_RTMP_PORT   = '1936'
    LIVE_SRS_HTTP_PORT   = '8088'
    LIVE_SRS_WEBRTC_HOST = '127.0.0.1'
    LIVE_SRS_WEBRTC_PORT = '8100'
    LIVE_SRS_API_BASE    = 'http://127.0.0.1:1986'
    LIVE_PUBLIC_BASE     = 'http://127.0.0.1:8088'
} -Exe (Join-Path $runtimeDir 'live-service-go.exe') -Arguments @('-config','configs/config.yaml')

Start-ServiceProcess -Name 'danmaku-service' -WorkDir (Join-Path $backendRoot 'danmaku-service-go') -EnvMap @{
    APP_ENV               = 'local'
    DANMAKU_MYSQL_DSN     = $dsnGo
    DANMAKU_REDIS_ADDR    = '127.0.0.1:6380'
    DANMAKU_RABBITMQ_URL  = "amqp://$($cfg['RABBITMQ_USER']):$($cfg['RABBITMQ_PASSWORD'])@127.0.0.1:5672/"
    DANMAKU_WS_ORIGINS    = '*'
} -Exe (Join-Path $runtimeDir 'danmaku-service-go.exe') -Arguments @('-config','configs/config.yaml')

Write-Host '[run] 启动 Java 服务 ...'
Start-ServiceProcess -Name 'user-service' -WorkDir (Join-Path $backendRoot 'user-service') -EnvMap @{
    SERVER_PORT            = '8001'
    MYSQL_URL              = $dsnJdbc
    MYSQL_USERNAME         = $dbUser
    MYSQL_PASSWORD         = $dbPw
    SPRING_SQL_INIT_MODE   = 'always'
    NACOS_SERVER_ADDR      = '127.0.0.1:8848'
    NACOS_REGISTRY_ADDRESS = 'nacos://127.0.0.1:8848'
    NACOS_USERNAME         = 'nacos'
    NACOS_PASSWORD         = 'nacos'
    DUBBO_PROTOCOL_PORT    = '-1'
    DUBBO_QOS_ENABLE       = 'false'
} -Exe 'java' -Arguments @('-jar','target\user-service-0.0.1-SNAPSHOT.jar')

Start-ServiceProcess -Name 'auth-service' -WorkDir (Join-Path $backendRoot 'auth-service') -EnvMap @{
    SERVER_PORT                  = '8002'
    SPRING_DATASOURCE_URL        = $dsnJdbc
    SPRING_DATASOURCE_USERNAME   = $dbUser
    SPRING_DATASOURCE_PASSWORD   = $dbPw
    SPRING_DATA_REDIS_HOST       = '127.0.0.1'
    SPRING_DATA_REDIS_PORT       = '6380'
    SPRING_SQL_INIT_MODE         = 'always'
    NACOS_SERVER_ADDR            = '127.0.0.1:8848'
    NACOS_USERNAME               = 'nacos'
    NACOS_PASSWORD               = 'nacos'
    DUBBO_REGISTRY_ADDRESS       = 'nacos://127.0.0.1:8848'
    JWT_SECRET                   = $jwtSecret
    AUTH_JWT_SECRET              = $jwtSecret
    AUTH_VERIFICATION_EXPOSE_CODE = 'true'
} -Exe 'java' -Arguments @('-jar','target\auth-service-0.0.1-SNAPSHOT.jar')

Start-ServiceProcess -Name 'product-service' -WorkDir (Join-Path $backendRoot 'product-service') -EnvMap @{
    SERVER_PORT                = '8003'
    SPRING_DATASOURCE_URL      = $dsnJdbc
    SPRING_DATASOURCE_USERNAME = $dbUser
    SPRING_DATASOURCE_PASSWORD = $dbPw
    NACOS_SERVER_ADDR          = '127.0.0.1:8848'
    NACOS_USERNAME             = 'nacos'
    NACOS_PASSWORD             = 'nacos'
    DUBBO_PROTOCOL_PORT        = '20880'
} -Exe 'java' -Arguments @('-jar','target\product-service-1.0.0.jar')

Start-ServiceProcess -Name 'order-service' -WorkDir (Join-Path $backendRoot 'order-service') -EnvMap @{
    SERVER_PORT                = '8004'
    SPRING_DATASOURCE_URL      = $dsnJdbc
    SPRING_DATASOURCE_USERNAME = $dbUser
    SPRING_DATASOURCE_PASSWORD = $dbPw
    NACOS_SERVER_ADDR          = '127.0.0.1:8848'
    NACOS_USERNAME             = 'nacos'
    NACOS_PASSWORD             = 'nacos'
    PRODUCT_RPC_URL            = 'dubbo://127.0.0.1:20880'
    MOCK_PAY_ENABLED           = 'true'
} -Exe 'java' -Arguments @('-jar','target\order-service-1.0.0.jar')

Start-ServiceProcess -Name 'gateway-service' -WorkDir (Join-Path $backendRoot 'gateway-service') -EnvMap @{
    SERVER_PORT                       = '8000'
    SPRING_DATA_REDIS_HOST            = '127.0.0.1'
    SPRING_DATA_REDIS_PORT            = '6380'
    NACOS_SERVER_ADDR                 = '127.0.0.1:8848'
    NACOS_USERNAME                    = 'nacos'
    NACOS_PASSWORD                    = 'nacos'
    JWT_SECRET                        = $jwtSecret
    VIDEO_SERVICE_URI                 = 'http://127.0.0.1:9001'
    LIVE_SERVICE_URI                  = 'http://127.0.0.1:9002'
    DANMAKU_SERVICE_URI               = 'http://127.0.0.1:9003'
    GATEWAY_RATE_LIMIT_GLOBAL_REPLENISH_RATE = '100000'
    GATEWAY_RATE_LIMIT_GLOBAL_BURST_CAPACITY = '100000'
    GATEWAY_RATE_LIMIT_IP_REPLENISH_RATE     = '1000'
    GATEWAY_RATE_LIMIT_IP_BURST_CAPACITY     = '1000'
} -Exe 'java' -Arguments @('-jar','target\gateway-service-0.0.1-SNAPSHOT.jar')

# ---------------------------------------------------------------- 等待端口
$ports = [ordered]@{
    'gateway'  = 8000
    'user'     = 8001
    'auth'     = 8002
    'product'  = 8003
    'order'    = 8004
    'video'    = 9001
    'live'     = 9002
    'danmaku'  = 9003
}

Write-Host '[wait] 等待服务端口 ...'
$deadline = (Get-Date).AddSeconds(180)
do {
    Start-Sleep -Seconds 3
    $pending = @()
    foreach ($name in $ports.Keys) {
        if (-not (Test-NetConnection -ComputerName 127.0.0.1 -Port $ports[$name] -InformationLevel Quiet -WarningAction SilentlyContinue)) {
            $pending += $name
        }
    }
} while ($pending.Count -gt 0 -and (Get-Date) -lt $deadline)

foreach ($name in $ports.Keys) {
    $ok = Test-NetConnection -ComputerName 127.0.0.1 -Port $ports[$name] -InformationLevel Quiet -WarningAction SilentlyContinue
    Write-Host ("  {0,-10} :{1} {2}" -f $name, $ports[$name], $(if ($ok) { 'OK' } else { 'FAILED（看 runtime/*.log）' }))
}

Write-Host ''
Write-Host '前端：cd web; $env:VITE_USE_MOCK="0"; pnpm dev'
Write-Host '停止：.\dev-down.ps1（加 -KeepInfra 保留基础设施）'
