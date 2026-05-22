CREATE TABLE plans (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                VARCHAR(100) NOT NULL,
    slug                VARCHAR(100) UNIQUE NOT NULL,
    description         TEXT,
    monthly_email_limit INT NOT NULL DEFAULT 1000,
    daily_email_limit   INT NOT NULL DEFAULT 100,
    rate_per_minute     INT NOT NULL DEFAULT 10,
    max_domains         INT NOT NULL DEFAULT 1,
    max_api_keys        INT NOT NULL DEFAULT 1,
    max_webhooks        INT NOT NULL DEFAULT 1,
    log_retention_days  INT NOT NULL DEFAULT 7,
    price_monthly_idr   INT NOT NULL DEFAULT 0,
    price_yearly_idr    INT NOT NULL DEFAULT 0,
    overage_per_1k_idr  INT NOT NULL DEFAULT 0,
    is_public           BOOLEAN DEFAULT TRUE,
    is_active           BOOLEAN DEFAULT TRUE,
    sort_order          INT DEFAULT 0,
    badge_text          VARCHAR(50),
    badge_color         VARCHAR(20),
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    updated_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE plan_features (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_id     UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    feature_key VARCHAR(100) NOT NULL,
    enabled     BOOLEAN DEFAULT TRUE,
    UNIQUE(plan_id, feature_key)
);

-- Hubungkan plans ke users sekarang
ALTER TABLE users ADD CONSTRAINT fk_users_plan FOREIGN KEY (current_plan_id) REFERENCES plans(id);
