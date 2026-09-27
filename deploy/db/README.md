# db —— 共用库 `auth_service` 的建表基线

Java 五个服务与 Go 三个服务**共用同一个 MySQL 实例、同一个库 `auth_service`**，
靠表名前缀区分归属（Java 侧的表没有前缀，Go 侧统一 `media_`）。

正因为是共库，DDL 才必须有一个**统一入口**：以前它散在 4 个 Java 服务的
`resources/` 里 + 仓库根的 `migrations/` 里，没有任何一处说明「新环境该按什么顺序建表」，
也没有任何一处说明「哪些是自动建的、哪些要手工导」。本文件补上这一块。

---

## 1. 全部表与归属

| 表 | 归属服务 | DDL 文件 | 会不会自动执行 |
| --- | --- | --- | --- |
| `account` | auth-service | [`../auth-service/src/main/resources/schema.sql`](../../auth-service/src/main/resources/schema.sql) | ✅ 自动 |
| `user` | user-service | [`../user-service/src/main/resources/schema.sql`](../../user-service/src/main/resources/schema.sql) | ✅ 自动 |
| `product` | product-service | [`../product-service/src/main/resources/db/schema.sql`](../../product-service/src/main/resources/db/schema.sql) | ❌ 手工 |
| `sku` | product-service | 同上 | ❌ 手工 |
| `inventory` | product-service | 同上 | ❌ 手工 |
| `order` | order-service | [`../order-service/src/main/resources/db/schema.sql`](../../order-service/src/main/resources/db/schema.sql) | ❌ 手工 |
| `order_item` | order-service | 同上 | ❌ 手工 |
| `payment_order` | order-service | 同上 | ❌ 手工 |
| `payment_record` | order-service | 同上 | ❌ 手工 |
| `media_video` | video-service-go | [`go/media-tables.sql`](go/media-tables.sql) | ❌ 手工 / GORM `AutoMigrate` |
| `media_video_category` | video-service-go | 同上 | 同上 |
| `media_video_tag` | video-service-go | 同上 | 同上 |
| `media_transcode_task` | video-service-go | 同上 | 同上 |
| `media_live_room` | live-service-go | 同上 | 同上 |
| `media_live_record` | live-service-go | 同上 | 同上 |
| `media_danmaku` | danmaku-service-go | 同上 | 同上 |
| `media_sensitive_word` | danmaku-service-go | 同上 | 同上 |

合计 **17 张表**：Java 9 张 + Go 8 张。

> Java 的 DDL **没有搬到这里**，是故意的：auth / user 的 `schema.sql` 必须留在
> classpath 根下（Spring 的 `spring.sql.init` 只从 classpath 找，且这两个服务配的是
> `mode: always`，每次启动都会执行一遍）。把它们复制一份过来就会变成两份副本，
> 迟早漂移 —— 这与 §5「DDL 漂移」那条教训正是同一类问题。

---

## 2. 新环境按这个顺序建表

```powershell
$mysql = "mysql --user=root --host=127.0.0.1 --port=3306 --default-character-set=utf8mb4"

# 0) 建库（如果还没有）
#    排序规则必须与生产/开发一致，否则字符串比较行为不同
mysql ... -e "CREATE DATABASE IF NOT EXISTS auth_service DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"

# 1) Go 侧 8 张表（幂等，可重复执行）
cmd /c "mysql ... auth_service < deploy\db\go\media-tables.sql"

# 2) Java 自动建的两张：不用管，启动 auth-service / user-service 时会自动执行
#    （spring.sql.init.mode=always → classpath:schema.sql）

# 3) Java 手工建的两组（product / order）
cmd /c "mysql ... auth_service < product-service\src\main\resources\db\schema.sql"
cmd /c "mysql ... auth_service < order-service\src\main\resources\db\schema.sql"
```

**两个必须注意的点：**

1. **`--default-character-set=utf8mb4` 不能省。** Windows 上 mysql 客户端默认按本地代码页
   解码文件，DDL 里的 UTF-8 中文注释会被解坏、吞掉后面的引号，报出看起来毫无关系的
   `ERROR 1064 ... near 'DRAFT'`。完整复现过程见
   [`../../Go微服务-问题解决记录.md`](../../Go微服务-问题解决记录.md) §6.2。

2. **喂文件用 `cmd /c "... < 文件"`，不要用 PowerShell 管道。** PS 管道会给 mysql 重新编码。

各文件的幂等性并不一致，重复执行前先看清楚：

| 文件 | 幂等？ |
| --- | --- |
| `auth-service/.../schema.sql` | ✅ `CREATE TABLE IF NOT EXISTS` |
| `user-service/.../schema.sql` | ✅ `CREATE TABLE IF NOT EXISTS` |
| `product-service/.../db/schema.sql` | ❌ 裸 `CREATE TABLE`，重复执行会报 `Table already exists` |
| `order-service/.../db/schema.sql` | ✅ `CREATE TABLE IF NOT EXISTS` |
| `deploy/db/go/media-tables.sql` | ✅ `CREATE TABLE IF NOT EXISTS` |

---

## 3. 用 compose 自建 MySQL 时

`deploy/compose.infra.yml` 的 MySQL 把 **`deploy/db/go/` 整个目录**挂进了
`docker-entrypoint-initdb.d`，所以**只有数据卷为空时首次初始化**会执行里面的
`media-tables.sql`：

- `media_*` 8 张表 → 由它自动建好；
- `account` / `user` → 等 auth / user 服务启动时由 Spring 建；
- `product` / `order` 那 7 张 → **仍需按上面第 3 步手工导入**（挂载目录里没有它们）。

`db/go/README.md` 是普通 Markdown，不会被 `docker-entrypoint-initdb` 当成脚本执行
（它只执行 `.sh` / `.sql` / `.sql.gz`）。

---

## 4. 校验：DDL 基线与代码是否已经漂移

Go 侧的表结构**权威来源是各服务 `pkg/migrate` 里的 GORM `AutoMigrate`**，
`db/go/media-tables.sql` 只是与它等价的、可人工审阅的副本。两者必须逐列逐索引一致。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File deploy/db/verify-schema.ps1
```

脚本会把 DDL 真跑进一个临时库 `media_ddl_check`，再逐表比对列与索引，最后删掉临时库。
期望输出 `通过 20 项，失败 0 项`。

**为什么必须在临时库里真跑一遍**：脚本比对的基线来自真实库的 `SHOW CREATE TABLE`，
而 `CREATE TABLE IF NOT EXISTS` 对着已存在的表执行是**空操作** ——
如果在原库里跑，无论 DDL 写得多离谱，「执行是否成功」永远为真，那样的校验毫无意义。
同理，改完 Go 的 model 后，不要凭 model 手写 DDL（历史上就是这么错的，详见
[`../../Go微服务-问题解决记录.md`](../../Go微服务-问题解决记录.md) §5），
要从真实库反向导出。

---

## 5. 历史坑（改 DDL 前值得先看）

- **Java 的 `db/schema.sql` 注释里还写着 `order_db` / `product_db`** —— 那是早期
  「一个服务一个库」方案的残留。现在五个 Java 服务的 `application.yml` 全部指向
  `auth_service`，不要被注释误导。
- `product-service` / `order-service` 的 `db/schema.sql` **不在** Spring 的默认扫描路径
  上（默认是 `classpath:schema.sql`），所以它们从来不会自动执行 —— 这就是它们需要
  「手工导入」的根本原因，不是配置漏了。
- `media_live_room` 的索引 `idx_media_live_room_deleted_at` 是 GORM 自动命名的**真实索引名**，
  不要改名，否则 `AutoMigrate` 会再建一个重复索引。
- `media-tables.sql` **不能带 UTF-8 BOM**，否则 BOM 会被当成第一条语句的一部分。
- Go 的 `int` / `int64` 都映射成 **BIGINT**（只有 `int32` 才是 `INT`），时间字段是 `DATETIME(3)`。

细节见 [`go/README.md`](go/README.md)。
