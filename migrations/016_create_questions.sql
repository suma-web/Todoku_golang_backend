CREATE TABLE IF NOT EXISTS question_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    group_id BIGINT NOT NULL REFERENCES school_groups(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS questions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id BIGINT NOT NULL REFERENCES question_categories(id) ON DELETE RESTRICT,
    title VARCHAR(200) NOT NULL CHECK (char_length(trim(title)) > 0),
    content TEXT NOT NULL CHECK (char_length(trim(content)) > 0),
    visibility VARCHAR(20) NOT NULL CHECK (visibility IN ('public', 'private')),
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'answered', 'resolved')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS question_answers (
    id BIGSERIAL PRIMARY KEY,
    question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL CHECK (char_length(trim(content)) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS questions_user_created_idx ON questions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS questions_category_status_idx ON questions(category_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS question_answers_question_idx ON question_answers(question_id, created_at, id);
