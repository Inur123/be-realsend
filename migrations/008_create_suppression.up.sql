CREATE TYPE suppression_reason AS ENUM ('hard_bounce', 'complaint', 'unsubscribe', 'manual');

CREATE TABLE suppression_list (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE, -- NULL = global
    email_address   VARCHAR(255) NOT NULL,
    reason          suppression_reason NOT NULL,
    note            TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, email_address)
);

CREATE INDEX idx_suppression_email ON suppression_list(email_address);
