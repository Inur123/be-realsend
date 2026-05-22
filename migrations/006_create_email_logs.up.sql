CREATE TYPE email_status AS ENUM (
    'queued',
    'processing',
    'sent',
    'delivered',
    'bounced',
    'rejected',
    'failed',
    'opened',
    'clicked'
);

CREATE TYPE bounce_type AS ENUM ('hard', 'soft', 'none');

CREATE TABLE email_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id      UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    domain_id       UUID REFERENCES domains(id) ON DELETE SET NULL,
    from_address    VARCHAR(255) NOT NULL,
    to_address      VARCHAR(255) NOT NULL,
    cc_addresses    TEXT[],
    bcc_addresses   TEXT[],
    subject         VARCHAR(1000),
    content_type    VARCHAR(20) DEFAULT 'text/html',
    status          email_status DEFAULT 'queued',
    bounce_type     bounce_type DEFAULT 'none',
    bounce_reason   TEXT,
    smtp_message_id VARCHAR(255),
    smtp_response   TEXT,
    opened_at       TIMESTAMPTZ,
    opened_count    INT DEFAULT 0,
    clicked_at      TIMESTAMPTZ,
    clicked_count   INT DEFAULT 0,
    tags            TEXT[],
    metadata        JSONB,
    headers         JSONB,
    queued_at       TIMESTAMPTZ DEFAULT NOW(),
    sent_at         TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_email_logs_user ON email_logs(user_id);
CREATE INDEX idx_email_logs_status ON email_logs(status);
CREATE INDEX idx_email_logs_created ON email_logs(created_at DESC);
CREATE INDEX idx_email_logs_to ON email_logs(to_address);
CREATE INDEX idx_email_logs_domain ON email_logs(domain_id);
CREATE INDEX idx_email_logs_tags ON email_logs USING GIN(tags);
CREATE INDEX idx_email_logs_metadata ON email_logs USING GIN(metadata);
