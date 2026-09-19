-- Active: 1725676203205@@127.0.0.1@3306@auth_service
CREATE TABLE IF NOT EXISTS `user` (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    uid VARCHAR(32) NOT NULL,
    username VARCHAR(32) NULL,
    phone VARCHAR(20),
    email VARCHAR(254),
    password_hash VARCHAR(100) NOT NULL,
    status TINYINT NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE KEY uk_phone (phone),
    UNIQUE KEY uk_email (email),
    UNIQUE KEY uk_uid (uid)
);
