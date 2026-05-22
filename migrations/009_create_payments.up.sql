CREATE TYPE payment_status AS ENUM ('pending', 'paid', 'failed', 'refunded', 'expired');

CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    plan_id         UUID REFERENCES plans(id) ON DELETE SET NULL,
    amount_idr      INT NOT NULL,
    payment_method  VARCHAR(50), -- 'midtrans', 'xendit'
    external_id     VARCHAR(255), -- ID from payment gateway
    status          payment_status DEFAULT 'pending',
    invoice_number  VARCHAR(50) UNIQUE,
    invoice_url     TEXT,
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_payments_user ON payments(user_id);
CREATE INDEX idx_payments_status ON payments(status);
