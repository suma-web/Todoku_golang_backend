ALTER TABLE school_posts
 ADD COLUMN previous_post_id BIGINT UNIQUE REFERENCES school_posts(id) ON DELETE RESTRICT,
 ADD COLUMN change_summary TEXT NOT NULL DEFAULT '',
 ADD CONSTRAINT school_posts_previous_older CHECK (previous_post_id IS NULL OR previous_post_id < id);
