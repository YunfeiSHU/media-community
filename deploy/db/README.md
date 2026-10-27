# db —— 共用库 `auth_service` 的建表基线

Java 五个服务与 Go 三个服务**共用同一个 MySQL 实例、同一个库 `auth_service`**，靠表名前缀区分归属：Java 侧 9 张无前缀，Go 侧 9 张统一 `media_` 前缀。

共库的前提是 DDL 只有一个入口，也就是本文件：一旦分散，「新环境按什么顺序建表」「哪些自动建、哪些要手工导」就没有任何一处说得清。部署总入口见 [`../README.md`](../README.md)。

---

## 1. 全部表与归属

| 表 | 归属服务 | DDL 文件 | 执行方式 |
| --- | --- | --- | --- |
| `account` | auth-service | [`auth-service/.../schema.sql`](../../auth-service/src/main/resources/schema.sql) | ✅ 自动 |
| `user` | user-service | [`user-service/.../schema.sql`](../../user-service/src/main/resources/schema.sql) | ✅ 自动 |
| `product` | product-service | [`product-service/.../db/schema.sql`](../../product-service/src/main/resources/db/schema.sql) | ❌ 手工 |
| `sku` | product-service | 同上 | ❌ 手工 |
| `inventory` | product-service | 同上 | ❌ 手工 |
| `order` | order-service | [`order-service/.../db/schema.sql`](../../order-service/src/main/resources/db/schema.sql) | ❌ 手工 |
| `order_item` | order-service | 同上 | ❌ 手工 |
| `payment_order` | order-service | 同上 | ❌ 手工 |
| `payment_record` | order-service | 同上 | ❌ 手工 |
| `media_video` | video-service-go | [`go/media-tables.sql`](go/media-tables.sql) | ❌ 手工 / GORM `AutoMigrate` |
| `media_video_category` | video-service-go | 同上 | 同上 |
| `media_video_tag` | video-service-go | 同上 | 同上 |
| `media_transcode_task` | video-service-go | 同上 | 同上 |
| `media_upload_session` | video-service-go | 同上 | 同上 |
| `media_live_room` | live-service-go | 同上 | 同上 |
| `media_live_record` | live-service-go | 同上 | 同上 |
| `media_danmaku` | danmaku-service-go | 同上 | 同上 |
| `media_sensitive_word` | danmaku-service-go | 同上 | 同上 |

合计 **18 张表**：Java 9 张 + Go 9 张。

> Java 的 DDL **没有搬到这里**，是故意的：auth / user 的 `schema.sql` 必须留在 classpath 根下（`spring.sql.init` 只从 classpath 找，且这两个服务配的是 `mode: always`，每次启动都执行）。复制一份过来就是两份副本，迟早漂移 —— 与 §6「三类漂移」是同一类问题。

---

## 2. 新环境按这个顺序建表

```powershell
# 连接参数固定成变量，下面拼进 cmd /c 的字符串里
$mysql = "mysql --user=root --host=127.0.0.1 --port=3306 --default-character-set=utf8mb4"

# 0) 建库（如果还没有）；排序规则必须与开发/生产一致，否则字符串比较行为不同
& mysql --user=root --host=127.0.0.1 --port=3306 --default-character-set=utf8mb4 -e "CREATE DATABASE IF NOT EXISTS auth_service DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"

# 1) Go 侧 9 张表（幂等，可重复执行）
cmd /c "$mysql auth_service < deploy\db\go\media-tables.sql"

# 2) Java 自动建的两张：不用管，启动 auth-service / user-service 时自动执行
#    （spring.sql.init.mode=always → classpath:schema.sql）

# 3) Java 手工建的两组（product / order）
cmd /c "$mysql auth_service < product-service\src\main\resources\db\schema.sql"
cmd /c "$mysql auth_service < order-service\src\main\resources\db\schema.sql"
```

两个必须注意的点：

1. **`--default-character-set=utf8mb4` 不能省。** Windows 上 mysql 客户端默认按本地代码页（GBK）解码文件，DDL 里的 UTF-8 中文注释会被误配对、吞掉后面的引号，报出看起来毫无关系的 `ERROR 1064 ... near 'DRAFT'`。根因是编码，不是类型或默认值写错。
2. **喂文件用 `cmd /c "... < 文件"`，不要用 PowerShell 管道**——管道会给 mysql 重新编码文本流。

幂等性并不一致，重复执行前先看清楚：

| 文件 | 幂等？ |
| --- | --- |
| `auth-service/.../schema.sql` | ✅ `CREATE TABLE IF NOT EXISTS` |
| `user-service/.../schema.sql` | ✅ `CREATE TABLE IF NOT EXISTS` |
| `product-service/.../db/schema.sql` | ❌ 裸 `CREATE TABLE`，重复执行报 `Table already exists` |
| `order-service/.../db/schema.sql` | ✅ `CREATE TABLE IF NOT EXISTS` |
| `deploy/db/go/media-tables.sql` | ✅ `CREATE TABLE IF NOT EXISTS`（且 `CREATE DATABASE IF NOT EXISTS` + `USE auth_service`） |

---

## 3. 用 compose 自建 MySQL 时

`deploy/compose.infra.yml` 把 **`deploy/db/go/` 整个目录**挂进 `docker-entrypoint-initdb.d`，只在**数据卷为空时的首次初始化**执行一次：

```yaml
volumes:
  - ./db/go:/docker-entrypoint-initdb.d:ro   # 相对 deploy/ 目录
```

- `media_*` 9 张 → 由挂载目录里的 `media-tables.sql` 自动建好；
- `account` / `user` → 等 auth / user 服务启动时由 Spring 建；
- `product` / `order` 那 7 张 → **仍需按 §2 第 3 步手工导入**（挂载目录里没有它们）。

两个约束：容器变量 `MYSQL_DATABASE` 必须保持 `auth_service`（DDL 里硬编码了 `CREATE DATABASE ... auth_service` 与 `USE auth_service`）；`docker-entrypoint-initdb.d` 只执行 `.sh` / `.sql` / `.sql.gz`，其他文件（含 Markdown）不会被执行，但也不要把非 DDL 的东西丢进这个目录。

---

## 4. Go 侧 DDL 基线与 `AutoMigrate` 的关系

| 角色 | 位置 | 说明 |
| --- | --- | --- |
| 权威来源 | `video-service-go` / `live-service-go` / `danmaku-service-go` 各自 `pkg/migrate` | 服务启动时在 `cmd/server/main.go` 调用 GORM `AutoMigrate`，只增不删；三个服务各管自己那几张表 |
| 可审阅副本 | [`go/media-tables.sql`](go/media-tables.sql) | 与 `AutoMigrate` 实际产出**逐列逐索引等价**，供人工审阅、预建表、容器初始化 |

两者必须一致，但**改完 model 后不要凭 model 手写 DDL**（历史上就是这么错的，见 §6），要从真实库 `SHOW CREATE TABLE` 反向导出。`AutoMigrate` 只增不删——手工 DDL 里多写的表/列不会被它清理，所以基线里每一行都得能追到 model。基线放在 `deploy/db/go/` 而不拆进各服务，原因只有一条：三个服务的表在同一个库，且 compose 要一次挂进 `docker-entrypoint-initdb.d`，拆开必然多份副本。

手工执行（幂等，可重复运行）：

```bash
# --default-character-set=utf8mb4 的原因同 §2：否则中文注释会吞掉引号，报 ERROR 1064
mysql -h127.0.0.1 -uroot -p --default-character-set=utf8mb4 auth_service < deploy/db/go/media-tables.sql
```

---

## 5. 校验：DDL 基线与代码是否已漂移

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File deploy/db/verify-schema.ps1
```

脚本把 DDL 里的 `USE auth_service` 改写成临时库 `media_ddl_check`，在临时库里**真跑一遍**建表，再逐表比对 `information_schema.COLUMNS`（列名/类型/可空/默认值/时间精度）与 `information_schema.STATISTICS`（索引名/唯一性/列序），最后删掉临时库。

期望输出 `通过 22 项，失败 0 项`（3 项前置：脚本执行无报错、表数一致、表名集合一致；9 张表 × 2 项列/索引；1 项临时库已删除）。手工低成本自查：`information_schema.COLUMNS` 中 `media_%` 表里 `COLUMN_TYPE IN ('int','int unsigned','datetime')` 的列应返回 **0 行**（残留即 §6 的漂移）。

**为什么必须在临时库里真跑一遍**：`CREATE TABLE IF NOT EXISTS` 对着已存在的表执行是**空操作**——在原库里跑，无论 DDL 写得多离谱，「执行是否成功」永远为真，那样的校验毫无意义。同理，肉眼比对也不可靠：两边必须导出成同构结果再逐行 diff。

---

## 6. 三类已知漂移与修法

`media-tables.sql` 曾按 model「想当然」手写，9 张表几乎全中三类系统性差异；根因是**命名暗示了列组合，而 GORM 的索引列完全由 tag 的 `priority` 决定**。

| 类别 | 手工 DDL 曾写成 | `AutoMigrate` 实际产出 | 修法 |
| --- | --- | --- | --- |
| 整型宽度 | `INT` | `BIGINT` | Go 的 `int` / `int64` 都是 64 位，只有 `int32` 映射 `INT`；DDL 一律写 `BIGINT` |
| 时间精度 | `DATETIME` | `DATETIME(3)` | GORM 时间字段默认毫秒精度，DDL 写 `DATETIME(3)` |
| 索引列 / 漏索引 | `idx_owner_created(owner_id, created_at)` | `idx_owner_created(owner_id)` + `idx_media_live_room_deleted_at(deleted_at)` | 索引列以 tag `priority` 为准（没标 `priority:2` 的就只有一列）；软删除字段的自动索引不能漏 |

修法统一：以 **`AutoMigrate` 为唯一事实来源** → `SHOW CREATE TABLE` 反向导出重写 DDL（collation `utf8mb4_0900_ai_ci`，MySQL 5.7 用 `utf8mb4_unicode_ci`，只影响排序语义）→ 跑 `verify-schema.ps1`。完整发现过程与 `information_schema` 比对 SQL 见 [`../../docs/Go微服务-问题解决记录.md`](../../docs/Go微服务-问题解决记录.md) §5.1。

---

## 7. 改 DDL 前的历史坑

- **Java 的 `db/schema.sql` 注释里还写着 `order_db` / `product_db`** —— 早期「一个服务一个库」方案的残留；五个 Java 服务的 `application.yml` 现在全部指向 `auth_service`，别被注释误导。
- `product-service` / `order-service` 的 `db/schema.sql` **不在** Spring 默认扫描路径（`classpath:schema.sql`）上，所以从来不会自动执行 —— 这才是「必须手工导入」的根本原因，不是配置漏了。
- `media_live_room` 的 `idx_media_live_room_deleted_at` 是 GORM 自动命名的**真实索引名**，不要改名；改名后 `AutoMigrate` 找不到原名会再建一个，同一列出现两个重复索引。想用自己的名字，必须在 model tag 里显式写 `index:你的名字`。
- `media-tables.sql` **不能带 UTF-8 BOM**，否则 BOM 会被当成第一条语句的一部分；`verify-schema.ps1` 读文件时也必须显式 `-Encoding UTF8`、写出时无 BOM，否则中文注释会被不可逆地转成 `?`，同样触发 `ERROR 1064`。

---

## 8. 演示种子数据

[`seed-media.sql`](seed-media.sql) 提供 12 条已发布视频 + 3 个直播中房间，用于前端首页/播放页在没有真实投稿时也能看到内容。

```powershell
cmd /c "mysql --user=root --host=127.0.0.1 --port=3307 --default-character-set=utf8mb4 auth_service < deploy\db\seed-media.sql"
```

- **幂等**：显式主键（9001–9012 / 9501–9503）+ `INSERT IGNORE`，可重复执行；
- 封面统一为空，由前端「标题首字 + 渐变」占位；等图片上传跑通后改成 `/media/image/...`；
- 商品与订单的种子数据随 S3（商城）阶段补。
