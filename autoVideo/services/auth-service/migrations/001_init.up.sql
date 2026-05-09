-- 001_init.up.sql
-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE,
    email         VARCHAR(128) UNIQUE,
    phone         VARCHAR(20)  UNIQUE,
    password_hash VARCHAR(256),
    avatar_url    TEXT,
    role          VARCHAR(32)  NOT NULL DEFAULT 'user',
    status        VARCHAR(16)  NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- OAuth 账号绑定表
CREATE TABLE IF NOT EXISTS oauth_accounts (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     VARCHAR(32) NOT NULL,
    provider_id  VARCHAR(128) NOT NULL,
    access_token TEXT,
    UNIQUE(provider, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_user_id ON oauth_accounts(user_id);

-- Refresh Token 表
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(256) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ  NOT NULL,
    device_info TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- 用户 API Key 表（存储第三方 AI 服务的密钥）
CREATE TABLE IF NOT EXISTS user_api_keys (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider      VARCHAR(64)  NOT NULL,
    key_alias     VARCHAR(128),
    encrypted_key TEXT         NOT NULL,
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_api_keys_user_id ON user_api_keys(user_id);

-- 权限表
CREATE TABLE IF NOT EXISTS permissions (
    id   BIGSERIAL PRIMARY KEY,
    code VARCHAR(128) NOT NULL UNIQUE,
    desc TEXT
);

-- 角色权限关联表
CREATE TABLE IF NOT EXISTS role_permissions (
    role       VARCHAR(32)  NOT NULL,
    permission VARCHAR(128) NOT NULL,
    PRIMARY KEY(role, permission)
);

-- 预置角色权限数据
INSERT INTO permissions (code, desc) VALUES
    ('video:create', '创建视频'),
    ('video:read',   '读取视频'),
    ('video:delete', '删除视频'),
    ('user:manage',  '管理用户'),
    ('apikey:manage','管理API Key')
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role, permission) VALUES
    ('admin', 'video:create'),
    ('admin', 'video:read'),
    ('admin', 'video:delete'),
    ('admin', 'user:manage'),
    ('admin', 'apikey:manage'),
    ('user', 'video:create'),
    ('user', 'video:read'),
    ('user', 'apikey:manage')
ON CONFLICT DO NOTHING;
