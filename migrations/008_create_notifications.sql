CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(20) NOT NULL,
    post_id BIGINT REFERENCES posts(id) ON DELETE CASCADE,
    comment_id BIGINT REFERENCES comments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT notifications_kind_valid
        CHECK (kind IN ('like', 'follow', 'comment'))
);

CREATE INDEX IF NOT EXISTS notifications_recipient_created_at_idx
    ON notifications (recipient_user_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS notifications_like_unique_idx
    ON notifications (actor_user_id, post_id)
    WHERE kind = 'like';

CREATE UNIQUE INDEX IF NOT EXISTS notifications_follow_unique_idx
    ON notifications (actor_user_id, recipient_user_id)
    WHERE kind = 'follow';

CREATE UNIQUE INDEX IF NOT EXISTS notifications_comment_unique_idx
    ON notifications (comment_id)
    WHERE kind = 'comment';

CREATE OR REPLACE FUNCTION create_like_notification()
RETURNS TRIGGER AS $$
DECLARE
    recipient_id BIGINT;
BEGIN
    SELECT user_id INTO recipient_id FROM posts WHERE id = NEW.post_id;
    IF recipient_id <> NEW.user_id THEN
        INSERT INTO notifications (
            recipient_user_id, actor_user_id, kind, post_id
        ) VALUES (recipient_id, NEW.user_id, 'like', NEW.post_id);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_like_notification()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM notifications
    WHERE kind = 'like'
      AND actor_user_id = OLD.user_id
      AND post_id = OLD.post_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION create_follow_notification()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO notifications (
        recipient_user_id, actor_user_id, kind
    ) VALUES (NEW.followed_id, NEW.follower_id, 'follow');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_follow_notification()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM notifications
    WHERE kind = 'follow'
      AND actor_user_id = OLD.follower_id
      AND recipient_user_id = OLD.followed_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION create_comment_notification()
RETURNS TRIGGER AS $$
DECLARE
    recipient_id BIGINT;
BEGIN
    SELECT user_id INTO recipient_id FROM posts WHERE id = NEW.post_id;
    IF recipient_id <> NEW.user_id THEN
        INSERT INTO notifications (
            recipient_user_id, actor_user_id, kind, post_id, comment_id
        ) VALUES (
            recipient_id, NEW.user_id, 'comment', NEW.post_id, NEW.id
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS likes_create_notification ON likes;
CREATE TRIGGER likes_create_notification
AFTER INSERT ON likes
FOR EACH ROW EXECUTE FUNCTION create_like_notification();

DROP TRIGGER IF EXISTS likes_delete_notification ON likes;
CREATE TRIGGER likes_delete_notification
AFTER DELETE ON likes
FOR EACH ROW EXECUTE FUNCTION delete_like_notification();

DROP TRIGGER IF EXISTS follows_create_notification ON follows;
CREATE TRIGGER follows_create_notification
AFTER INSERT ON follows
FOR EACH ROW EXECUTE FUNCTION create_follow_notification();

DROP TRIGGER IF EXISTS follows_delete_notification ON follows;
CREATE TRIGGER follows_delete_notification
AFTER DELETE ON follows
FOR EACH ROW EXECUTE FUNCTION delete_follow_notification();

DROP TRIGGER IF EXISTS comments_create_notification ON comments;
CREATE TRIGGER comments_create_notification
AFTER INSERT ON comments
FOR EACH ROW EXECUTE FUNCTION create_comment_notification();
