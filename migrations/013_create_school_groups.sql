CREATE TABLE IF NOT EXISTS school_groups (
 id BIGSERIAL PRIMARY KEY, name VARCHAR(100) NOT NULL,
 type VARCHAR(20) NOT NULL CHECK (type IN ('grade','class','club','committee','department')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(name,type)
);
CREATE TABLE IF NOT EXISTS user_school_groups (
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 group_id BIGINT NOT NULL REFERENCES school_groups(id) ON DELETE CASCADE,
 PRIMARY KEY(user_id,group_id)
);
CREATE INDEX IF NOT EXISTS user_school_groups_group_idx ON user_school_groups(group_id,user_id);
