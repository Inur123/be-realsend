ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS location;

ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS user_agent;
