CREATE TABLE IF NOT EXISTS bookmarks (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT bookmarks_post_user_unique UNIQUE (post_id, user_id)
);

CREATE INDEX IF NOT EXISTS bookmarks_user_created_at_idx
    ON bookmarks (user_id, created_at DESC);
