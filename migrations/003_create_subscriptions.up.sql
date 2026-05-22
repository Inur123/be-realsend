CREATE TYPE subscription_status AS ENUM ('active', 'expired', 'cancelled', 'past_due');

CREATE TABLE subscriptions (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id                 UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id                 UUID NOT NULL REFERENCES plans(id),
    status                  subscription_status DEFAULT 'active',
    started_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at              TIMESTAMPTZ,
    cancelled_at            TIMESTAMPTZ,
    billing_cycle           VARCHAR(20) DEFAULT 'monthly',
    amount_idr              INT NOT NULL DEFAULT 0,
    payment_method          VARCHAR(50),
    emails_sent_this_month  INT DEFAULT 0,
    emails_sent_today       INT DEFAULT 0,
    month_reset_at          TIMESTAMPTZ DEFAULT NOW(),
    day_reset_at            TIMESTAMPTZ DEFAULT NOW(),
    created_at              TIMESTAMPTZ DEFAULT NOW(),
    updated_at              TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE user_plan_overrides (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature_key     VARCHAR(100) NOT NULL,
    override_value  VARCHAR(255) NOT NULL,
    note            TEXT,
    created_by      UUID REFERENCES users(id),
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, feature_key)
);

CREATE INDEX idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
