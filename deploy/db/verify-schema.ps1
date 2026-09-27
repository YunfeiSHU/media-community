# 校验 db/go/media-tables.sql 与 GORM AutoMigrate 实际产出的结构完全一致
#
# 做法：把脚本里的 USE `auth_service` 改成临时库，在临时库里跑一遍建表，
# 然后逐表比较 information_schema 中的列与索引，最后删掉临时库。
# 注意：脚本本身是 CREATE TABLE IF NOT EXISTS，直接在 auth_service 上跑是空操作，
# 起不到校验作用，所以必须换库执行。

$ErrorActionPreference = "Stop"
$env:MYSQL_PWD = '***REMOVED***'
# 脚本与 DDL 在同一个目录树里，用 $PSScriptRoot 定位，不依赖当前工作目录：
#   deploy/db/verify-schema.ps1  ← 本脚本
#   deploy/db/go/media-tables.sql ← 被校验的基线
$src = Join-Path $PSScriptRoot "go\media-tables.sql"
$tmpDb = "media_ddl_check"
$tmp = Join-Path $env:TEMP "media-ddl-check.sql"
# --default-character-set=utf8mb4 必不可少：Windows 上 mysql 客户端默认按本地代码页（GBK）
# 解码 SQL 文件，UTF-8 中文注释的字节会被误配对（如 `'秒'` 的尾字节 + `'` 被当成一个 GBK 汉字），
# 结果吞掉引号导致报 `ERROR 1064 ... near 'DRAFT'`——看起来像类型/默认值写错，实际是编码问题。
$mysql = @("mysql", "--user=root", "--host=127.0.0.1", "--port=3306", "--default-character-set=utf8mb4")

function Q([string]$db, [string]$sql) {
    return (& mysql @mysql "--database=$db" "-N" "-B" "-e" $sql)
}

# 注意：MySQL 客户端会按字节解析 SQL 文件，**不能带 UTF-8 BOM**，
# 而且必须用 cmd 的 `<` 重定向（PowerShell 管道会重新编码文本流）。
function RunSqlFile([string]$file) {
    & cmd /c "mysql --user=root --host=127.0.0.1 --port=3306 --default-character-set=utf8mb4 < `"$file`""
    return $LASTEXITCODE
}

$script:pass = 0; $script:fail = 0
function Check([string]$name, [bool]$ok, [string]$detail) {
    if ($ok) { $script:pass++; Write-Host ("  [PASS] " + $name) -ForegroundColor Green }
    else { $script:fail++; Write-Host ("  [FAIL] " + $name + "  -> " + $detail) -ForegroundColor Red }
}

Write-Host "`n[1] 在临时库 $tmpDb 中执行 DDL 脚本" -ForegroundColor Cyan
# -Encoding UTF8 不能省：PS 5.1 的 Get-Content 默认按 ANSI（中文系统上是 GBK）解码，
# 会把 UTF-8 的中文注释读成乱码且不可逆（非法字节变 '?'），
# 再写回文件时字节就变了，进而可能吞掉引号 → mysql 报 `ERROR 1064 ... near 'DRAFT'`。
$content = (Get-Content $src -Raw -Encoding UTF8) -replace 'USE `auth_service`;', "USE ``$tmpDb``;"
# 无 BOM 写入，否则 BOM 会被当成第一条语句的一部分
[System.IO.File]::WriteAllText($tmp, $content, [System.Text.UTF8Encoding]::new($false))
& mysql @mysql "-e" "DROP DATABASE IF EXISTS $tmpDb; CREATE DATABASE $tmpDb DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"
$code = RunSqlFile $tmp
Check "脚本执行无报错" ($code -eq 0) "mysql exit=$code"

$liveTables = @(Q "auth_service" "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA='auth_service' AND TABLE_NAME LIKE 'media_%' ORDER BY TABLE_NAME;")
$tmpTables = @(Q $tmpDb "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA='$tmpDb' AND TABLE_NAME LIKE 'media_%' ORDER BY TABLE_NAME;")
Check "临时库建出同样多的表" ($tmpTables.Count -eq $liveTables.Count) "live=$($liveTables.Count) tmp=$($tmpTables.Count)"
Check "表名集合完全一致" ((($liveTables -join ',') -eq ($tmpTables -join ','))) "live=[$($liveTables -join ',')] tmp=[$($tmpTables -join ',')]"

Write-Host "`n[2] 逐表比较列定义" -ForegroundColor Cyan
$colSql = "SELECT CONCAT(COLUMN_NAME,':',COLUMN_TYPE,':',IS_NULLABLE,':',IFNULL(COLUMN_DEFAULT,'~'),':',IFNULL(DATETIME_PRECISION,'~')) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='##DB##' AND TABLE_NAME='##T##' ORDER BY ORDINAL_POSITION;"
$idxSql = "SELECT CONCAT(INDEX_NAME,':',NON_UNIQUE,':',GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX)) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='##DB##' AND TABLE_NAME='##T##' GROUP BY INDEX_NAME, NON_UNIQUE ORDER BY INDEX_NAME;"
foreach ($t in $liveTables) {
    $a = @(Q "auth_service" ($colSql -replace '##DB##', 'auth_service' -replace '##T##', $t))
    $b = @(Q $tmpDb ($colSql -replace '##DB##', $tmpDb -replace '##T##', $t))
    $same = ($a -join '|') -eq ($b -join '|')
    Check "$t 列定义一致" $same ("`n    live=[" + ($a -join ',') + "]`n    tmp =[" + ($b -join ',') + "]")

    $ia = @(Q "auth_service" ($idxSql -replace '##DB##', 'auth_service' -replace '##T##', $t))
    $ib = @(Q $tmpDb ($idxSql -replace '##DB##', $tmpDb -replace '##T##', $t))
    $sameIdx = ($ia -join '|') -eq ($ib -join '|')
    Check "$t 索引定义一致" $sameIdx ("`n    live=[" + ($ia -join ',') + "]`n    tmp =[" + ($ib -join ',') + "]")
}

Write-Host "`n[3] 清理临时库" -ForegroundColor Cyan
& mysql @mysql "-e" "DROP DATABASE IF EXISTS $tmpDb;"
$still = @(Q "auth_service" "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='$tmpDb';")
Check "临时库已删除" ($still.Count -eq 0) ($still -join ',')
Remove-Item $tmp -Force -ErrorAction SilentlyContinue

Write-Host "`n========== 结果：通过 $script:pass 项，失败 $script:fail 项 ==========" -ForegroundColor Cyan
if ($script:fail -gt 0) { exit 1 }
Write-Host "DDL 文件与 AutoMigrate 结构完全一致" -ForegroundColor Green
