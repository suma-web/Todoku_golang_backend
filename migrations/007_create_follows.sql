CREATE TABLE IF NOT EXISTS follows (
    id BIGSERIAL PRIMARY KEY,
    follower_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followed_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT follows_users_different CHECK (follower_id <> followed_id),
    CONSTRAINT follows_follower_followed_unique UNIQUE (follower_id, followed_id)
);

CREATE INDEX IF NOT EXISTS follows_followed_id_idx
    ON follows (followed_id);
