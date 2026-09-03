-- Remove the unused social-network schema retained from the original prototype.
-- Historical migrations remain intact so existing databases can upgrade safely.

DROP TABLE IF EXISTS bookmarks;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS retweets;
DROP TABLE IF EXISTS likes;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS follows;
DROP TABLE IF EXISTS posts;

DROP FUNCTION IF EXISTS create_like_notification();
DROP FUNCTION IF EXISTS delete_like_notification();
DROP FUNCTION IF EXISTS create_follow_notification();
DROP FUNCTION IF EXISTS delete_follow_notification();
DROP FUNCTION IF EXISTS create_comment_notification();
DROP FUNCTION IF EXISTS create_retweet_notification();
DROP FUNCTION IF EXISTS delete_retweet_notification();
