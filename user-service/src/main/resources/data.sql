INSERT INTO `user` (uid, nickname, avatar, gender, bio, status, created_at, updated_at)
SELECT 'demo', '演示用户', NULL, NULL, 'user-service demo account', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM `user` WHERE uid = 'demo');
