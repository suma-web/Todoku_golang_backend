CREATE TABLE IF NOT EXISTS retweets (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT retweets_post_user_unique UNIQUE (post_id, user_id)
);

CREATE INDEX IF NOT EXISTS retweets_post_id_idx
    ON retweets (post_id);
