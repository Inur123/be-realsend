CREATE TYPE domain_status AS ENUM ('pending', 'verified', 'failed', 'expired');

CREATE TABLE domains (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    domain_name       VARCHAR(255) NOT NULL,
    status            domain_status DEFAULT 'pending',
    spf_record        TEXT,
    dkim_selector     VARCHAR(100),
    dkim_public_key   TEXT,
    dkim_private_key  TEXT, -- encrypted
    dmarc_record      TEXT,
    return_path_cname VARCHAR(255),
    spf_verified      BOOLEAN DEFAULT FALSE,
    dkim_verified     BOOLEAN DEFAULT FALSE,
    dmarc_verified    BOOLEAN DEFAULT FALSE,
    last_verified_at  TIMESTAMPTZ,
    verified_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ DEFAULT NOW(),
    updated_at        TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, domain_name)
);

CREATE INDEX idx_domains_user ON domains(user_id);
CREATE INDEX idx_domains_status ON domains(status);
CREATE INDEX idx_domains_name ON domains(domain_name);
