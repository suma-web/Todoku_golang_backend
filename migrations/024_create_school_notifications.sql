-- The legacy notifications table was removed by migration 021.
CREATE TABLE notifications (
    id BIGSERIAL PRIMARY KEY,
    recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    post_id BIGINT NOT NULL REFERENCES school_posts(id) ON DELETE CASCADE,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('important_update')),
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- No unique constraint on recipient/post: each notified edit is a new event.
CREATE INDEX notifications_recipient_created_idx ON notifications(recipient_user_id,created_at DESC,id DESC);
CREATE INDEX notifications_unread_idx ON notifications(recipient_user_id) WHERE read_at IS NULL;
