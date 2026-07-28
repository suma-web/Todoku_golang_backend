CREATE TABLE IF NOT EXISTS comments (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment VARCHAR(140) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT comments_content_required
        CHECK (char_length(trim(comment)) > 0)
);

CREATE INDEX IF NOT EXISTS comments_post_created_at_idx
    ON comments (post_id, created_at, id);