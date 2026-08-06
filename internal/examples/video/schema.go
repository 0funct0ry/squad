package video

const Schema = `-- Minimal video sharing schema
PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    avatar_url      TEXT,
    country_code    TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1))
);

CREATE INDEX idx_users_email ON users(email);

CREATE TABLE channels (
    channel_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_user_id    INTEGER NOT NULL,
    channel_name     TEXT NOT NULL,
    handle           TEXT NOT NULL UNIQUE,      -- e.g. @mychannel
    description      TEXT,
    banner_url       TEXT,
    avatar_url       TEXT,
    subscriber_count INTEGER NOT NULL DEFAULT 0,
    video_count      INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (owner_user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_channels_owner ON channels(owner_user_id);
CREATE INDEX idx_channels_handle ON channels(handle);

CREATE TABLE categories (
    category_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    category_name   TEXT NOT NULL UNIQUE
);

CREATE TABLE videos (
    video_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id       INTEGER NOT NULL,
    category_id      INTEGER,
    title            TEXT NOT NULL,
    description      TEXT,
    file_url         TEXT NOT NULL,
    thumbnail_url    TEXT,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    visibility       TEXT NOT NULL DEFAULT 'public'
                        CHECK (visibility IN ('public','unlisted','private')),
    status           TEXT NOT NULL DEFAULT 'processing'
                        CHECK (status IN ('processing','ready','failed','removed')),
    is_made_for_kids INTEGER NOT NULL DEFAULT 0 CHECK (is_made_for_kids IN (0,1)),
    published_at     TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (channel_id) REFERENCES channels(channel_id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(category_id) ON DELETE SET NULL
);

CREATE INDEX idx_videos_channel ON videos(channel_id);
CREATE INDEX idx_videos_category ON videos(category_id);
CREATE INDEX idx_videos_published ON videos(published_at);

CREATE TABLE video_stats (
    video_id        INTEGER PRIMARY KEY,
    view_count      INTEGER NOT NULL DEFAULT 0,
    like_count      INTEGER NOT NULL DEFAULT 0,
    dislike_count   INTEGER NOT NULL DEFAULT 0,
    comment_count   INTEGER NOT NULL DEFAULT 0,
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (video_id) REFERENCES videos(video_id) ON DELETE CASCADE
);

CREATE TABLE tags (
    tag_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_name    TEXT NOT NULL UNIQUE
);

CREATE TABLE video_tags (
    video_id    INTEGER NOT NULL,
    tag_id      INTEGER NOT NULL,
    PRIMARY KEY (video_id, tag_id),
    FOREIGN KEY (video_id) REFERENCES videos(video_id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(tag_id) ON DELETE CASCADE
);

CREATE INDEX idx_video_tags_tag ON video_tags(tag_id);

CREATE TABLE playlists (
    playlist_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id      INTEGER NOT NULL,
    title           TEXT NOT NULL,
    description     TEXT,
    visibility      TEXT NOT NULL DEFAULT 'public'
                        CHECK (visibility IN ('public','unlisted','private')),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (channel_id) REFERENCES channels(channel_id) ON DELETE CASCADE
);

CREATE INDEX idx_playlists_channel ON playlists(channel_id);

CREATE TABLE playlist_videos (
    playlist_id     INTEGER NOT NULL,
    video_id        INTEGER NOT NULL,
    position        INTEGER NOT NULL,
    added_at        TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (playlist_id, video_id),
    FOREIGN KEY (playlist_id) REFERENCES playlists(playlist_id) ON DELETE CASCADE,
    FOREIGN KEY (video_id) REFERENCES videos(video_id) ON DELETE CASCADE,
    UNIQUE (playlist_id, position)
);

CREATE INDEX idx_playlist_videos_video ON playlist_videos(video_id);

CREATE TABLE subscriptions (
    subscriber_user_id  INTEGER NOT NULL,
    channel_id          INTEGER NOT NULL,
    notifications_on    INTEGER NOT NULL DEFAULT 1 CHECK (notifications_on IN (0,1)),
    subscribed_at       TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (subscriber_user_id, channel_id),
    FOREIGN KEY (subscriber_user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES channels(channel_id) ON DELETE CASCADE
);

CREATE INDEX idx_subscriptions_channel ON subscriptions(channel_id);

CREATE TABLE comments (
    comment_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id            INTEGER NOT NULL,
    user_id             INTEGER NOT NULL,
    parent_comment_id   INTEGER,
    body                TEXT NOT NULL,
    like_count          INTEGER NOT NULL DEFAULT 0,
    is_pinned           INTEGER NOT NULL DEFAULT 0 CHECK (is_pinned IN (0,1)),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    is_deleted          INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1)),
    FOREIGN KEY (video_id) REFERENCES videos(video_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (parent_comment_id) REFERENCES comments(comment_id) ON DELETE CASCADE
);

CREATE INDEX idx_comments_video ON comments(video_id);
CREATE INDEX idx_comments_user ON comments(user_id);
CREATE INDEX idx_comments_parent ON comments(parent_comment_id);

CREATE TABLE comment_reactions (
    comment_id      INTEGER NOT NULL,
    user_id         INTEGER NOT NULL,
    reaction_type   TEXT NOT NULL CHECK (reaction_type IN ('like','dislike')),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (comment_id, user_id),
    FOREIGN KEY (comment_id) REFERENCES comments(comment_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE video_reactions (
    video_id        INTEGER NOT NULL,
    user_id         INTEGER NOT NULL,
    reaction_type   TEXT NOT NULL CHECK (reaction_type IN ('like','dislike')),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (video_id, user_id),
    FOREIGN KEY (video_id) REFERENCES videos(video_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_video_reactions_user ON video_reactions(user_id);

CREATE TABLE watch_history (
    watch_id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id             INTEGER NOT NULL,
    video_id            INTEGER NOT NULL,
    watched_at          TEXT NOT NULL DEFAULT (datetime('now')),
    progress_seconds    INTEGER NOT NULL DEFAULT 0,
    completed           INTEGER NOT NULL DEFAULT 0 CHECK (completed IN (0,1)),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (video_id) REFERENCES videos(video_id) ON DELETE CASCADE
);

CREATE INDEX idx_watch_history_user ON watch_history(user_id, watched_at DESC);
CREATE INDEX idx_watch_history_video ON watch_history(video_id);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE user_id = NEW.user_id;
END;

CREATE TRIGGER trg_videos_updated_at
AFTER UPDATE ON videos
BEGIN
    UPDATE videos SET updated_at = datetime('now') WHERE video_id = NEW.video_id;
END;

CREATE TRIGGER trg_channels_updated_at
AFTER UPDATE ON channels
BEGIN
    UPDATE channels SET updated_at = datetime('now') WHERE channel_id = NEW.channel_id;
END;

CREATE TRIGGER trg_video_stats_init
AFTER INSERT ON videos
BEGIN
    INSERT INTO video_stats (video_id) VALUES (NEW.video_id);
END;
`
