-- Seed default plans
INSERT INTO plans (id, name, slug, description, monthly_email_limit, daily_email_limit, rate_per_minute, max_domains, max_api_keys, price_monthly_idr, sort_order) VALUES
('a0000000-0000-0000-0000-000000000001', 'Free',      'free',      'Gratis selamanya untuk testing & proyek kecil.', 1000,     100,    10,   1,  1,  0,       1),
('a0000000-0000-0000-0000-000000000002', 'Starter',   'starter',   'Ideal untuk startup & developer mandiri.', 50000,    5000,   100,  3,  3,  79000,   2),
('a0000000-0000-0000-0000-000000000003', 'Growth',    'growth',    'Untuk bisnis yang sedang berkembang pesat.', 200000,   20000,  500,  5,  5,  249000,  3),
('a0000000-0000-0000-0000-000000000004', 'Pro',       'pro',       'Performa maksimal dengan fitur enterprise lengkap.', 1000000,  100000, 2000, 10, 10, 799000,  4);

-- Seed features for Free plan
INSERT INTO plan_features (plan_id, feature_key, enabled) VALUES
('a0000000-0000-0000-0000-000000000001', 'smtp_auth', true),
('a0000000-0000-0000-0000-000000000001', 'email_tracking', false),
('a0000000-0000-0000-0000-000000000001', 'webhook_events', false),
('a0000000-0000-0000-0000-000000000001', 'template_engine', false),
('a0000000-0000-0000-0000-000000000001', 'dedicated_ip', false),
('a0000000-0000-0000-0000-000000000001', 'priority_queue', false);

-- Seed features for Starter plan
INSERT INTO plan_features (plan_id, feature_key, enabled) VALUES
('a0000000-0000-0000-0000-000000000002', 'smtp_auth', true),
('a0000000-0000-0000-0000-000000000002', 'email_tracking', true),
('a0000000-0000-0000-0000-000000000002', 'open_tracking', true),
('a0000000-0000-0000-0000-000000000002', 'click_tracking', true),
('a0000000-0000-0000-0000-000000000002', 'webhook_events', true),
('a0000000-0000-0000-0000-000000000002', 'template_engine', true),
('a0000000-0000-0000-0000-000000000002', 'dedicated_ip', false),
('a0000000-0000-0000-0000-000000000002', 'priority_queue', false);

-- Seed features for Growth plan
INSERT INTO plan_features (plan_id, feature_key, enabled) VALUES
('a0000000-0000-0000-0000-000000000003', 'smtp_auth', true),
('a0000000-0000-0000-0000-000000000003', 'email_tracking', true),
('a0000000-0000-0000-0000-000000000003', 'open_tracking', true),
('a0000000-0000-0000-0000-000000000003', 'click_tracking', true),
('a0000000-0000-0000-0000-000000000003', 'webhook_events', true),
('a0000000-0000-0000-0000-000000000003', 'template_engine', true),
('a0000000-0000-0000-0000-000000000003', 'dedicated_ip', false),
('a0000000-0000-0000-0000-000000000003', 'priority_queue', true);

-- Seed features for Pro plan
INSERT INTO plan_features (plan_id, feature_key, enabled) VALUES
('a0000000-0000-0000-0000-000000000004', 'smtp_auth', true),
('a0000000-0000-0000-0000-000000000004', 'email_tracking', true),
('a0000000-0000-0000-0000-000000000004', 'open_tracking', true),
('a0000000-0000-0000-0000-000000000004', 'click_tracking', true),
('a0000000-0000-0000-0000-000000000004', 'webhook_events', true),
('a0000000-0000-0000-0000-000000000004', 'template_engine', true),
('a0000000-0000-0000-0000-000000000004', 'dedicated_ip', true),
('a0000000-0000-0000-0000-000000000004', 'priority_queue', true),
('a0000000-0000-0000-0000-000000000004', 'email_scheduling', true),
('a0000000-0000-0000-0000-000000000004', 'suppression_list', true),
('a0000000-0000-0000-0000-000000000004', 'analytics_advanced', true),
('a0000000-0000-0000-0000-000000000004', 'api_batch_send', true),
('a0000000-0000-0000-0000-000000000004', 'custom_return_path', true),
('a0000000-0000-0000-0000-000000000004', 'subaccount', true);
