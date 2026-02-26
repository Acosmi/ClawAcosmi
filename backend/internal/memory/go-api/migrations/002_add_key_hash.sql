-- 迁移脚本：为 api_keys 表添加 key_hash 列（SHA-256 哈希存储）
-- 执行方式：psql -U uhms -d nexus_v4 -f 002_add_key_hash.sql
-- 1. 添加 key_hash 列
ALTER TABLE api_keys
ADD COLUMN IF NOT EXISTS key_hash VARCHAR(64);
-- 2. 为已有 key 计算哈希（PostgreSQL 内置 SHA-256）
UPDATE api_keys
SET key_hash = encode(sha256(key::bytea), 'hex')
WHERE key IS NOT NULL
    AND (
        key_hash IS NULL
        OR key_hash = ''
    );
-- 3. 为 key_hash 列创建索引（加速查询）
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys (key_hash);
-- 4. 注意：暂不删除 key 列，保留兼容过渡期
-- 后续可在确认所有 key 迁移完成后执行:
-- ALTER TABLE api_keys DROP COLUMN key;