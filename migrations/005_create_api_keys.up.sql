CREATE TABLE api_keys (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          VARCHAR(255) NOT NULL,
    key_prefix    VARCHAR(10) NOT NULL,
    key_hash      VARCHAR(255) NOT NULL,
    last_4        VARCHAR(4) NOT NULL,
    scopes        TEXT[] DEFAULT '{"email:send"}',
    domain_id     UUID REFERENCES domains(id) ON DELETE SET NULL,
    is_active     BOOLEAN DEFAULT TRUE,
    last_used_at  TIMESTAMPTZ,
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);
