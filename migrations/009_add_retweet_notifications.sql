ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_kind_valid;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_valid
        CHECK (kind IN ('like', 'follow', 'comment', 'retweet'));

CREATE UNIQUE INDEX IF NOT EXISTS notifications_retweet_unique_idx
    ON notifications (actor_user_id, post_id)
    WHERE kind = 'retweet';

CREATE OR REPLACE FUNCTION create_retweet_notification()
RETURNS TRIGGER AS $$
DECLARE
    recipient_id BIGINT;
BEGIN
    SELECT user_id INTO recipient_id FROM posts WHERE id = NEW.post_id;
    IF recipient_id <> NEW.user_id THEN
        INSERT INTO notifications (
            recipient_user_id, actor_user_id, kind, post_id
        ) VALUES (recipient_id, NEW.user_id, 'retweet', NEW.post_id);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION delete_retweet_notification()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM notifications
    WHERE kind = 'retweet'
      AND actor_user_id = OLD.user_id
      AND post_id = OLD.post_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS retweets_create_notification ON retweets;
CREATE TRIGGER retweets_create_notification
AFTER INSERT ON retweets
FOR EACH ROW EXECUTE FUNCTION create_retweet_notification();

DROP TRIGGER IF EXISTS retweets_delete_notification ON retweets;
CREATE TRIGGER retweets_delete_notification
AFTER DELETE ON retweets
FOR EACH ROW EXECUTE FUNCTION delete_retweet_notification();
