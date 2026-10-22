-- 说明：5 个 Java 服务共用同一个库 auth_service，靠表名前缀区分（Java 服务表名不带 media_ 前缀）。
-- gender 用 VARCHAR(16)（如 MALE / FEMALE / OTHER），与领域模型（String gender）及对客契约一致；
-- bio 用 VARCHAR(500)，与 DTO 的 @Size(max = 500) 对齐。
-- 若库里已有旧表（gender TINYINT / bio VARCHAR(255)），需手工执行：
--   ALTER TABLE `user` MODIFY COLUMN gender VARCHAR(16) NULL, MODIFY COLUMN bio VARCHAR(500) NULL;
CREATE TABLE IF NOT EXISTS `user` (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    uid VARCHAR(32) NOT NULL UNIQUE,
    nickname VARCHAR(64),
    avatar VARCHAR(512),
    gender VARCHAR(16),
    bio VARCHAR(500),
    role TINYINT NOT NULL DEFAULT 1,
    status TINYINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);