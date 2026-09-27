# media-tables.sql —— Go 侧建表基线

## 这是什么

`media-tables.sql` 是 **Go 媒体微服务群**（video / live / danmaku）的建表 DDL 基线，
描述 **`auth_service` 库**（与 Java 体系共用的同一个库）中 `media_` 前缀的全部 8 张表。

**完整的表清单与「新环境建表顺序」在上级文档 [`../README.md`](../README.md)**，
这里只讲这份文件本身的使用方式与约束。

## 为什么放在 `deploy/db/go/`，而不是各服务内

三个服务的表在**同一个数据库**里，`deploy/compose.infra.yml` 又需要把整份 DDL
挂进 `docker-entrypoint-initdb.d` 一次性初始化。
放在一处是**唯一**不会产生多份副本 / 漂移的放法：

```yaml
volumes:
  - ./db/go:/docker-entrypoint-initdb.d:ro   # 相对 deploy/ 目录
```

## 事实来源（重要）

**表的权威来源是各服务 `pkg/migrate` 里的 GORM `AutoMigrate`**，
本文件是与之等价的、可人工审阅的 DDL。两者必须逐列 / 逐索引一致。

改 model 后请同步本文件，并用校验脚本确认：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File deploy/db/verify-schema.ps1
```

脚本会在临时库里真跑一遍本 DDL，再逐表比对列与索引，最后删掉临时库。
期望输出 `通过 20 项，失败 0 项`。

手工执行（幂等，可重复运行）：

```bash
# 注意 --default-character-set=utf8mb4：否则 Windows 上 mysql 客户端按本地代码页解码，
# UTF-8 中文注释会吞掉后面的引号，报出看起来毫不相关的 ERROR 1064
mysql -h127.0.0.1 -uroot -p --default-character-set=utf8mb4 auth_service < deploy/db/go/media-tables.sql
```

## 注意事项

- 文件**不能带 UTF-8 BOM**（否则 BOM 会被当成第一条语句的一部分）。
- `media_live_room` 的 `idx_media_live_room_deleted_at` 是 GORM 自动命名的**真实索引名**，
  不要改名 —— 改名后 `AutoMigrate` 会再建一个重复索引。
- Go 的 `int` / `int64` 都映射成 `BIGINT`（只有 `int32` 才是 `INT`）；
  GORM 时间字段是 `DATETIME(3)`。

## 历史教训

本文件**曾经是错的**，且错得有系统性（`INT` vs `BIGINT`、`DATETIME` vs `DATETIME(3)`、
索引列凭名字臆测、漏掉 `idx_media_live_room_deleted_at`），原因是"照着 model 手写"
而不是"从真实库反向导出"。

完整的发现过程、`information_schema` 比对 SQL、以及为什么校验脚本**必须在临时库里真跑一遍**
（因为 `CREATE TABLE IF NOT EXISTS` 对着已有表执行是空操作，那样的校验永远通过），
见 **`../../../docs/Go微服务-问题解决记录.md` §5**。
