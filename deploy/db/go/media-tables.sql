-- ---------------------------------------------------------------------------
-- Go 媒体微服务群 建表脚本
--
-- 目标库：auth_service（与 Java 体系共用的同一个库）
--
-- 【重要】本文件的字段类型 / 精度 / 索引已与「GORM AutoMigrate 实际产出的结构」
-- 逐列对齐（在 MySQL 8.0 上实测）。三条容易写错的规则：
--   1) Go 的 int / int64 都映射成 MySQL BIGINT（只有 int32 才是 INT）；
--   2) GORM 的时间字段是 DATETIME(3)，不是 DATETIME（毫秒精度）；
--   3) 索引名以 model tag 里的 index: 名字为准，列以 priority 为准；
--      例如 idx_owner_created 只有 owner_id 一列（model 里没给 created_at 标 priority:2），
--      不要凭名字臆测成 (owner_id, created_at)。
--
-- 表名统一使用 media_ 前缀，原因：
--   1) 该库中已有 Java 侧的表：account / user / product / sku / inventory /
--      order / order_item / payment_order / payment_record（均为无前缀的通用名）；
--   2) 共库场景下，无前缀的 video / danmaku 这类名字无法表达归属，
--      且未来 Java 侧一旦新增同名表就会冲突；
--   3) GORM AutoMigrate 会自动建表，本脚本的用途是：
--      - 自建中间件（cd deploy; docker compose --profile infra up -d）时的容器初始化
--        （compose.infra.yml 把 deploy/db/go/ 挂进了 docker-entrypoint-initdb.d）；
--      - 需要手工审阅 / 预建表时直接执行（幂等，可重复运行）。
--
-- 执行方式（两种都行）：
--   mysql -h127.0.0.1 -uroot -p auth_service < deploy/db/go/media-tables.sql
--   或直接启动任一 Go 服务，由 AutoMigrate 建表
--
-- 字符集说明：下面按 MySQL 8 的 AutoMigrate 实测结果写 utf8mb4_0900_ai_ci。
-- 若要在 MySQL 5.7 上执行，把该排序规则全部替换为 utf8mb4_unicode_ci 即可
-- （只影响排序/比较语义，不影响字段与索引的正确性）。
--
-- 校验本文件是否仍与实际库一致（应返回 0 行）：
--   SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, DATETIME_PRECISION
--     FROM information_schema.COLUMNS
--    WHERE TABLE_SCHEMA='auth_service' AND TABLE_NAME LIKE 'media_%'
--      AND COLUMN_TYPE IN ('int','int unsigned','datetime');
-- ---------------------------------------------------------------------------

-- 若目标库不存在则创建（独立部署时会用到；连 Java 的实例时该库已存在）
CREATE DATABASE IF NOT EXISTS `auth_service`
  DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;

USE `auth_service`;

-- ===========================================================================
-- video-service-go
-- ===========================================================================

CREATE TABLE IF NOT EXISTS `media_video` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `title`         VARCHAR(128)  NOT NULL,
  `description`   VARCHAR(1024) NOT NULL DEFAULT '',
  `owner_id`      VARCHAR(64)   NOT NULL COMMENT '来自 X-User-Id',
  `category_id`   BIGINT UNSIGNED NULL,
  `cover_url`     VARCHAR(512)  NOT NULL DEFAULT '',
  `source_object` VARCHAR(512)  NOT NULL DEFAULT '' COMMENT 'MinIO 对象 key',
  `playback_url`  VARCHAR(512)  NOT NULL DEFAULT '' COMMENT 'HLS master.m3u8 地址',
  `duration`      BIGINT        NOT NULL DEFAULT 0 COMMENT '秒',
  `size_bytes`    BIGINT        NOT NULL DEFAULT 0,
  `status`        VARCHAR(16)   NOT NULL DEFAULT 'DRAFT',
  `view_count`    BIGINT        NOT NULL DEFAULT 0,
  `like_count`     BIGINT        NOT NULL DEFAULT 0,
  `favorite_count` BIGINT        NOT NULL DEFAULT 0,
  `published_at`  DATETIME(3)   NULL,
  `created_at`    DATETIME(3)   NULL,
  `updated_at`    DATETIME(3)   NULL,
  `deleted_at`    DATETIME(3)   NULL,
  PRIMARY KEY (`id`),
  KEY `idx_owner_created` (`owner_id`),
  KEY `idx_category` (`category_id`),
  KEY `idx_status_created` (`status`),
  KEY `idx_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='点播视频元数据';

CREATE TABLE IF NOT EXISTS `media_video_category` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name`       VARCHAR(64) NOT NULL,
  `sort`       BIGINT      NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_category_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='点播分类';

CREATE TABLE IF NOT EXISTS `media_video_tag` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `video_id`   BIGINT UNSIGNED NOT NULL,
  `tag`        VARCHAR(32) NOT NULL,
  `created_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_video_tag` (`video_id`, `tag`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频标签';

CREATE TABLE IF NOT EXISTS `media_transcode_task` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `video_id`    BIGINT UNSIGNED NOT NULL,
  `event_id`    VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'MQ 事件 ID，用于幂等与排障',
  `status`      VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  `profiles`    VARCHAR(255) NOT NULL DEFAULT '' COMMENT '360p,720p,1080p',
  `progress`    BIGINT      NOT NULL DEFAULT 0,
  `message`     VARCHAR(512) NOT NULL DEFAULT '',
  `started_at`  DATETIME(3) NULL,
  `finished_at` DATETIME(3) NULL,
  `created_at`  DATETIME(3) NULL,
  `updated_at`  DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_video` (`video_id`),
  KEY `idx_event` (`event_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频转码任务';

-- 分片上传会话：断点续传的锚点。
-- 已上传分片的**权威数据在对象存储侧**（ListParts），本表只需要记住
-- 「videoId → uploadId」这个映射，以及切片计划（part_size / total_parts）。
CREATE TABLE IF NOT EXISTS `media_upload_session` (
  `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `video_id`    BIGINT UNSIGNED NOT NULL,
  `owner_id`    VARCHAR(64)  NOT NULL COMMENT '来自 X-User-Id',
  `object_key`  VARCHAR(512) NOT NULL COMMENT '原片在对象存储中的 key，同一视频的会话共用同一个 key',
  `upload_id`   VARCHAR(255) NOT NULL COMMENT '对象存储侧的 multipart uploadId，断点续传的唯一入口',
  `ext`         VARCHAR(16)  NOT NULL,
  `file_size`   BIGINT       NOT NULL DEFAULT 0 COMMENT '客户端声明的原片总字节数',
  `part_size`   BIGINT       NOT NULL DEFAULT 0 COMMENT '分片大小（字节），init 时由服务端算好',
  `total_parts` BIGINT       NOT NULL DEFAULT 0 COMMENT '分片总数 = ceil(file_size / part_size)',
  `status`      VARCHAR(16)  NOT NULL DEFAULT 'UPLOADING' COMMENT 'UPLOADING|COMPLETED|ABORTED',
  `created_at`  DATETIME(3)  NULL,
  `updated_at`  DATETIME(3)  NULL,
  `deleted_at`  DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  KEY `idx_video` (`video_id`),
  KEY `idx_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='分片上传会话（断点续传）';

-- ===========================================================================
-- live-service-go
-- ===========================================================================

CREATE TABLE IF NOT EXISTS `media_live_room` (
  `id`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `owner_id`        VARCHAR(64)  NOT NULL COMMENT '来自 X-User-Id（主播）',
  `title`           VARCHAR(64)  NOT NULL,
  `notice`          VARCHAR(512) NOT NULL DEFAULT '',
  `cover_url`       VARCHAR(512) NOT NULL DEFAULT '',
  `category_id`     BIGINT UNSIGNED NULL,
  `stream_key`      VARCHAR(64)  NOT NULL COMMENT '服务端生成，每次开播重新生成',
  `status`          VARCHAR(16)  NOT NULL DEFAULT 'IDLE',
  `current_session` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '本次直播会话 ID',
  `viewer_total`    BIGINT       NOT NULL DEFAULT 0 COMMENT '累计观看人次（人气值）',
  `peak_online`     BIGINT       NOT NULL DEFAULT 0,
  `last_started_at` DATETIME(3)  NULL,
  `last_ended_at`   DATETIME(3)  NULL,
  `created_at`      DATETIME(3)  NULL,
  `updated_at`      DATETIME(3)  NULL,
  `deleted_at`      DATETIME(3)  NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_stream_key` (`stream_key`),
  KEY `idx_owner` (`owner_id`),
  KEY `idx_owner_status` (`owner_id`, `status`),
  KEY `idx_status_updated` (`status`, `updated_at`),
  -- 注意：这个索引名是 GORM 自动生成的（model 里 DeletedAt 没写 index: 名字），
  -- 不要「顺手改名」——改名后 AutoMigrate 会再建一个新索引，导致出现两个重复索引。
  KEY `idx_media_live_room_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='直播间';

CREATE TABLE IF NOT EXISTS `media_live_record` (
  `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `room_id`      BIGINT UNSIGNED NOT NULL,
  `session_id`   VARCHAR(64) NOT NULL,
  `started_at`   DATETIME(3) NOT NULL,
  `ended_at`     DATETIME(3) NULL,
  `duration_sec` BIGINT      NOT NULL DEFAULT 0,
  `peak_online`  BIGINT      NOT NULL DEFAULT 0,
  `viewer_total` BIGINT      NOT NULL DEFAULT 0,
  `end_reason`   VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'manual|unpublish|force|timeout',
  `created_at`   DATETIME(3) NULL,
  `updated_at`   DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session` (`session_id`),
  KEY `idx_room_started` (`room_id`, `started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='直播场次记录';

-- ===========================================================================
-- danmaku-service-go
-- ===========================================================================

CREATE TABLE IF NOT EXISTS `media_danmaku` (
  `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `room_id`       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '直播弹幕所属房间；点播为空',
  `video_id`      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '点播弹幕所属视频；直播为空',
  `uid`           VARCHAR(64)  NOT NULL COMMENT '来自 X-User-Id',
  `nickname`      VARCHAR(64)  NOT NULL DEFAULT '',
  `content`       VARCHAR(200) NOT NULL,
  `color`         VARCHAR(9)   NOT NULL DEFAULT '#FFFFFF',
  `position`      VARCHAR(16)  NOT NULL DEFAULT 'SCROLL',
  `time_ms`       BIGINT       NOT NULL DEFAULT 0 COMMENT '点播时间轴位置（毫秒）',
  `client_msg_id` VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '客户端幂等键',
  `status`        VARCHAR(16)  NOT NULL DEFAULT 'NORMAL' COMMENT 'NORMAL|RECALLED',
  `sent_at`       DATETIME(3)  NOT NULL,
  `created_at`    DATETIME(3)  NOT NULL COMMENT '弹幕没有 updated_at：写入后只改 status，不需要',
  PRIMARY KEY (`id`),
  KEY `idx_room_id_id` (`room_id`, `id`),
  KEY `idx_room_sent` (`room_id`, `sent_at`),
  KEY `idx_video_time` (`video_id`, `time_ms`),
  KEY `idx_uid_sent` (`uid`, `sent_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='弹幕';

CREATE TABLE IF NOT EXISTS `media_sensitive_word` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `word`       VARCHAR(64) NOT NULL,
  `strategy`   VARCHAR(16) NOT NULL DEFAULT 'REJECT' COMMENT 'REJECT|REPLACE|LOG',
  `enabled`    TINYINT(1)  NOT NULL DEFAULT 1,
  `created_at` DATETIME(3) NOT NULL,
  `updated_at` DATETIME(3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_word` (`word`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='弹幕敏感词';

-- ---------------------------------------------------------------- S4 互动
-- 点赞 / 收藏关系。唯一键让「重复点击」在数据库层天然幂等：
-- 插入冲突时忽略，RowsAffected=0，计数不会被重复累加。
CREATE TABLE IF NOT EXISTS `media_video_like` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `video_id`   BIGINT UNSIGNED NOT NULL,
  `uid`        VARCHAR(64)     NOT NULL,
  `created_at` DATETIME(3)     NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_like_video_uid` (`video_id`, `uid`),
  KEY `idx_like_uid_created` (`uid`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频点赞';

CREATE TABLE IF NOT EXISTS `media_video_favorite` (
  `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `video_id`   BIGINT UNSIGNED NOT NULL,
  `uid`        VARCHAR(64)     NOT NULL,
  `created_at` DATETIME(3)     NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_favorite_video_uid` (`video_id`, `uid`),
  KEY `idx_favorite_uid_created` (`uid`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='视频收藏';
