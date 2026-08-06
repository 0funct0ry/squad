package media

const Schema = `-- Minimal visual social network schema
PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    username            TEXT NOT NULL UNIQUE,
    email               TEXT NOT NULL UNIQUE,
    phone_number        TEXT UNIQUE,
    password_hash       TEXT NOT NULL,
    full_name           TEXT,
    bio                 TEXT,
    profile_picture_url TEXT,
    website_url         TEXT,
    is_private          INTEGER NOT NULL DEFAULT 0 CHECK (is_private IN (0,1)),
    is_verified         INTEGER NOT NULL DEFAULT 0 CHECK (is_verified IN (0,1)),
    is_business_account INTEGER NOT NULL DEFAULT 0 CHECK (is_business_account IN (0,1)),
    is_active           INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);

CREATE TABLE user_sessions (
    session_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL,
    device_info     TEXT,
    ip_address      TEXT,
    login_at        TEXT NOT NULL DEFAULT (datetime('now')),
    logout_at       TEXT,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_sessions_user ON user_sessions(user_id);

CREATE TABLE follows (
    follow_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    follower_id     INTEGER NOT NULL,   -- the user who initiates the follow
    followee_id     INTEGER NOT NULL,   -- the user being followed
    status          TEXT NOT NULL DEFAULT 'accepted' CHECK (status IN ('pending','accepted','rejected')),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (follower_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (followee_id) REFERENCES users(user_id) ON DELETE CASCADE,
    UNIQUE (follower_id, followee_id),
    CHECK (follower_id != followee_id)
);

CREATE INDEX idx_follows_follower ON follows(follower_id);
CREATE INDEX idx_follows_followee ON follows(followee_id);
CREATE INDEX idx_follows_status ON follows(status);

CREATE TABLE blocks (
    block_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    blocker_id      INTEGER NOT NULL,
    blocked_id      INTEGER NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (blocker_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (blocked_id) REFERENCES users(user_id) ON DELETE CASCADE,
    UNIQUE (blocker_id, blocked_id),
    CHECK (blocker_id != blocked_id)
);

CREATE INDEX idx_blocks_blocker ON blocks(blocker_id);
CREATE INDEX idx_blocks_blocked ON blocks(blocked_id);


CREATE TABLE posts (
    post_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id             INTEGER NOT NULL,
    caption             TEXT,
    location_name       TEXT,
    latitude            REAL,
    longitude           REAL,
    is_archived         INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0,1)),
    comments_disabled   INTEGER NOT NULL DEFAULT 0 CHECK (comments_disabled IN (0,1)),
    like_count          INTEGER NOT NULL DEFAULT 0,
    comment_count       INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_posts_user ON posts(user_id);
CREATE INDEX idx_posts_created ON posts(created_at);

CREATE TABLE post_media (
    media_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id         INTEGER NOT NULL,
    media_type      TEXT NOT NULL CHECK (media_type IN ('image','video')),
    media_url       TEXT NOT NULL,
    thumbnail_url   TEXT,
    duration_secs   REAL,               -- for videos
    width           INTEGER,
    height          INTEGER,
    display_order   INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (post_id) REFERENCES posts(post_id) ON DELETE CASCADE
);

CREATE INDEX idx_post_media_post ON post_media(post_id, display_order);

CREATE TABLE post_tags (
    post_tag_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id         INTEGER NOT NULL,
    tagged_user_id  INTEGER NOT NULL,
    x_position      REAL,   -- normalized 0-1 position on image
    y_position      REAL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (post_id) REFERENCES posts(post_id) ON DELETE CASCADE,
    FOREIGN KEY (tagged_user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    UNIQUE (post_id, tagged_user_id)
);

CREATE INDEX idx_post_tags_user ON post_tags(tagged_user_id);


CREATE TABLE hashtags (
    hashtag_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_name        TEXT NOT NULL UNIQUE,
    post_count      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_hashtags_name ON hashtags(tag_name);

CREATE TABLE post_hashtags (
    post_id         INTEGER NOT NULL,
    hashtag_id      INTEGER NOT NULL,
    PRIMARY KEY (post_id, hashtag_id),
    FOREIGN KEY (post_id) REFERENCES posts(post_id) ON DELETE CASCADE,
    FOREIGN KEY (hashtag_id) REFERENCES hashtags(hashtag_id) ON DELETE CASCADE
);

CREATE INDEX idx_post_hashtags_hashtag ON post_hashtags(hashtag_id);


CREATE TABLE stories (
    story_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL,
    media_type      TEXT NOT NULL CHECK (media_type IN ('image','video')),
    media_url       TEXT NOT NULL,
    caption         TEXT,
    duration_secs   REAL DEFAULT 5,
    view_count      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at      TEXT NOT NULL DEFAULT (datetime('now', '+24 hours')),
    is_highlight    INTEGER NOT NULL DEFAULT 0 CHECK (is_highlight IN (0,1)),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_stories_user ON stories(user_id);
CREATE INDEX idx_stories_expires ON stories(expires_at);

CREATE TABLE story_views (
    story_view_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    story_id        INTEGER NOT NULL,
    viewer_id       INTEGER NOT NULL,
    viewed_at       TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (story_id) REFERENCES stories(story_id) ON DELETE CASCADE,
    FOREIGN KEY (viewer_id) REFERENCES users(user_id) ON DELETE CASCADE,
    UNIQUE (story_id, viewer_id)
);

CREATE INDEX idx_story_views_story ON story_views(story_id);
CREATE INDEX idx_story_views_viewer ON story_views(viewer_id);

CREATE TABLE highlights (
    highlight_id    INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL,
    title           TEXT NOT NULL,
    cover_image_url TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE highlight_stories (
    highlight_id    INTEGER NOT NULL,
    story_id        INTEGER NOT NULL,
    display_order   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (highlight_id, story_id),
    FOREIGN KEY (highlight_id) REFERENCES highlights(highlight_id) ON DELETE CASCADE,
    FOREIGN KEY (story_id) REFERENCES stories(story_id) ON DELETE CASCADE
);

CREATE TABLE comments (
    comment_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id             INTEGER NOT NULL,
    user_id             INTEGER NOT NULL,
    parent_comment_id   INTEGER,           -- NULL = top-level, else reply
    content             TEXT NOT NULL,
    like_count          INTEGER NOT NULL DEFAULT 0,
    is_edited           INTEGER NOT NULL DEFAULT 0 CHECK (is_edited IN (0,1)),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (post_id) REFERENCES posts(post_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (parent_comment_id) REFERENCES comments(comment_id) ON DELETE CASCADE
);

CREATE INDEX idx_comments_post ON comments(post_id);
CREATE INDEX idx_comments_user ON comments(user_id);
CREATE INDEX idx_comments_parent ON comments(parent_comment_id);

CREATE TABLE likes (
    like_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL,
    target_type     TEXT NOT NULL CHECK (target_type IN ('post','comment','story')),
    target_id       INTEGER NOT NULL,   -- references posts.post_id, comments.comment_id, or stories.story_id
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    UNIQUE (user_id, target_type, target_id)
);

CREATE INDEX idx_likes_target ON likes(target_type, target_id);
CREATE INDEX idx_likes_user ON likes(user_id);


CREATE TABLE conversations (
    conversation_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    is_group            INTEGER NOT NULL DEFAULT 0 CHECK (is_group IN (0,1)),
    group_name          TEXT,               -- NULL for 1:1 conversations
    group_icon_url      TEXT,
    created_by          INTEGER,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (created_by) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX idx_conversations_updated ON conversations(updated_at);

CREATE TABLE conversation_participants (
    conversation_id     INTEGER NOT NULL,
    user_id             INTEGER NOT NULL,
    joined_at           TEXT NOT NULL DEFAULT (datetime('now')),
    left_at             TEXT,
    is_admin            INTEGER NOT NULL DEFAULT 0 CHECK (is_admin IN (0,1)),
    is_muted            INTEGER NOT NULL DEFAULT 0 CHECK (is_muted IN (0,1)),
    PRIMARY KEY (conversation_id, user_id),
    FOREIGN KEY (conversation_id) REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_conv_participants_user ON conversation_participants(user_id);

CREATE TABLE messages (
    message_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id      INTEGER NOT NULL,
    sender_id            INTEGER NOT NULL,
    message_type         TEXT NOT NULL DEFAULT 'text' CHECK (message_type IN ('text','image','video','shared_post','shared_story','audio')),
    content               TEXT,               -- text content, or NULL for pure media
    media_url             TEXT,               -- for image/video/audio messages
    shared_post_id        INTEGER,            -- when sharing a post via DM
    is_deleted            INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1)),
    is_edited             INTEGER NOT NULL DEFAULT 0 CHECK (is_edited IN (0,1)),
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at            TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (conversation_id) REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    FOREIGN KEY (sender_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (shared_post_id) REFERENCES posts(post_id) ON DELETE SET NULL
);

CREATE INDEX idx_messages_conversation ON messages(conversation_id, created_at);
CREATE INDEX idx_messages_sender ON messages(sender_id);

CREATE TABLE message_reads (
    conversation_id     INTEGER NOT NULL,
    user_id             INTEGER NOT NULL,
    last_read_message_id INTEGER,
    read_at              TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (conversation_id, user_id),
    FOREIGN KEY (conversation_id) REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (last_read_message_id) REFERENCES messages(message_id) ON DELETE SET NULL
);

CREATE TABLE message_reactions (
    message_reaction_id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id           INTEGER NOT NULL,
    user_id               INTEGER NOT NULL,
    emoji                 TEXT NOT NULL,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (message_id) REFERENCES messages(message_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    UNIQUE (message_id, user_id, emoji)
);

CREATE INDEX idx_message_reactions_message ON message_reactions(message_id);


CREATE TABLE notifications (
    notification_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    recipient_id         INTEGER NOT NULL,
    actor_id             INTEGER,            -- user who triggered the notification
    notification_type    TEXT NOT NULL CHECK (notification_type IN
                          ('like','comment','follow_request','follow_accept','mention','new_message','tag')),
    target_type          TEXT CHECK (target_type IN ('post','comment','story','user','message')),
    target_id             INTEGER,
    is_read               INTEGER NOT NULL DEFAULT 0 CHECK (is_read IN (0,1)),
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (recipient_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (actor_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_notifications_recipient ON notifications(recipient_id, is_read);
CREATE INDEX idx_notifications_created ON notifications(created_at);


CREATE TRIGGER trg_likes_insert_post
AFTER INSERT ON likes
WHEN NEW.target_type = 'post'
BEGIN
    UPDATE posts SET like_count = like_count + 1 WHERE post_id = NEW.target_id;
END;

CREATE TRIGGER trg_likes_delete_post
AFTER DELETE ON likes
WHEN OLD.target_type = 'post'
BEGIN
    UPDATE posts SET like_count = like_count - 1 WHERE post_id = OLD.target_id;
END;

CREATE TRIGGER trg_likes_insert_comment
AFTER INSERT ON likes
WHEN NEW.target_type = 'comment'
BEGIN
    UPDATE comments SET like_count = like_count + 1 WHERE comment_id = NEW.target_id;
END;

CREATE TRIGGER trg_likes_delete_comment
AFTER DELETE ON likes
WHEN OLD.target_type = 'comment'
BEGIN
    UPDATE comments SET like_count = like_count - 1 WHERE comment_id = OLD.target_id;
END;

CREATE TRIGGER trg_comments_insert
AFTER INSERT ON comments
BEGIN
    UPDATE posts SET comment_count = comment_count + 1 WHERE post_id = NEW.post_id;
END;

CREATE TRIGGER trg_comments_delete
AFTER DELETE ON comments
BEGIN
    UPDATE posts SET comment_count = comment_count - 1 WHERE post_id = OLD.post_id;
END;

CREATE TRIGGER trg_story_views_insert
AFTER INSERT ON story_views
BEGIN
    UPDATE stories SET view_count = view_count + 1 WHERE story_id = NEW.story_id;
END;

CREATE TRIGGER trg_post_hashtags_insert
AFTER INSERT ON post_hashtags
BEGIN
    UPDATE hashtags SET post_count = post_count + 1 WHERE hashtag_id = NEW.hashtag_id;
END;

CREATE TRIGGER trg_post_hashtags_delete
AFTER DELETE ON post_hashtags
BEGIN
    UPDATE hashtags SET post_count = post_count - 1 WHERE hashtag_id = OLD.hashtag_id;
END;

CREATE TRIGGER trg_messages_insert_touch_conversation
AFTER INSERT ON messages
BEGIN
    UPDATE conversations SET updated_at = datetime('now') WHERE conversation_id = NEW.conversation_id;
END;

CREATE TRIGGER trg_users_update_touch
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE user_id = NEW.user_id;
END;

CREATE TRIGGER trg_posts_update_touch
AFTER UPDATE OF caption, location_name, is_archived, comments_disabled ON posts
BEGIN
    UPDATE posts SET updated_at = datetime('now') WHERE post_id = NEW.post_id;
END;


CREATE VIEW v_post_feed_base AS
SELECT
    p.post_id,
    p.user_id AS author_id,
    u.username AS author_username,
    u.profile_picture_url AS author_avatar,
    p.caption,
    p.like_count,
    p.comment_count,
    p.created_at
FROM posts p
JOIN users u ON u.user_id = p.user_id
WHERE p.is_archived = 0;

CREATE VIEW v_active_stories AS
SELECT
    s.story_id,
    s.user_id,
    u.username,
    s.media_url,
    s.media_type,
    s.view_count,
    s.created_at,
    s.expires_at
FROM stories s
JOIN users u ON u.user_id = s.user_id
WHERE s.expires_at > datetime('now');

CREATE VIEW v_user_follow_counts AS
SELECT
    u.user_id,
    u.username,
    (SELECT COUNT(*) FROM follows f WHERE f.followee_id = u.user_id AND f.status = 'accepted') AS follower_count,
    (SELECT COUNT(*) FROM follows f WHERE f.follower_id = u.user_id AND f.status = 'accepted') AS following_count,
    (SELECT COUNT(*) FROM posts p WHERE p.user_id = u.user_id AND p.is_archived = 0) AS post_count
FROM users u;

CREATE VIEW v_unread_message_counts AS
SELECT
    cp.conversation_id,
    cp.user_id,
    COUNT(m.message_id) AS unread_count
FROM conversation_participants cp
JOIN messages m ON m.conversation_id = cp.conversation_id
LEFT JOIN message_reads mr ON mr.conversation_id = cp.conversation_id AND mr.user_id = cp.user_id
WHERE m.sender_id != cp.user_id
  AND (mr.last_read_message_id IS NULL OR m.message_id > mr.last_read_message_id)
GROUP BY cp.conversation_id, cp.user_id;
`
