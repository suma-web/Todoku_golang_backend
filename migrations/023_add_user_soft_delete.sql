ALTER TABLE users
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS users_active_not_deleted_idx
    ON users (role, is_active)
    WHERE deleted_at IS NULL;
