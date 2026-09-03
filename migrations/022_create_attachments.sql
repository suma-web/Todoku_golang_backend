CREATE TABLE IF NOT EXISTS attachments (
    id BIGSERIAL PRIMARY KEY,
    uploaded_by BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    school_post_id BIGINT REFERENCES school_posts(id) ON DELETE CASCADE,
    question_id BIGINT REFERENCES questions(id) ON DELETE CASCADE,
    answer_id BIGINT REFERENCES question_answers(id) ON DELETE CASCADE,
    original_name VARCHAR(255) NOT NULL,
    storage_key VARCHAR(500) NOT NULL UNIQUE,
    content_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT attachments_one_parent CHECK (
        num_nonnulls(school_post_id, question_id, answer_id) = 1
    )
);

CREATE INDEX IF NOT EXISTS attachments_school_post_idx
    ON attachments (school_post_id, created_at, id) WHERE school_post_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS attachments_question_idx
    ON attachments (question_id, created_at, id) WHERE question_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS attachments_answer_idx
    ON attachments (answer_id, created_at, id) WHERE answer_id IS NOT NULL;
