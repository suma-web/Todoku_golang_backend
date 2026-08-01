CREATE TABLE IF NOT EXISTS likes (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT likes_post_user_unique UNIQUE (post_id, user_id)
);

CREATE INDEX IF NOT EXISTS likes_post_id_idx
    ON likes (post_id);
