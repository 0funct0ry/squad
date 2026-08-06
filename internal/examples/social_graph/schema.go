package social_graph

const Schema = `-- Minimal social graph schema
PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    date_of_birth   DATE,
    gender          TEXT CHECK (gender IN ('male','female','other','prefer_not_to_say')),
    profile_photo_url TEXT,
    bio             TEXT,
    location        TEXT,
    status          TEXT NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active','deactivated','suspended','deleted')),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    last_active_at  TEXT
);

CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_last_active ON users(last_active_at);

CREATE TABLE friend_requests (
    request_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    requester_id    INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    addressee_id    INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','accepted','declined','cancelled','blocked')),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    responded_at    TEXT,
    CHECK (requester_id <> addressee_id),
    UNIQUE (requester_id, addressee_id)
);

CREATE INDEX idx_friend_requests_addressee ON friend_requests(addressee_id, status);
CREATE INDEX idx_friend_requests_requester ON friend_requests(requester_id, status);

CREATE TABLE friendships (
    friendship_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id_low     INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    user_id_high    INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    source_request_id INTEGER REFERENCES friend_requests(request_id) ON DELETE SET NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (user_id_low < user_id_high),
    UNIQUE (user_id_low, user_id_high)
);

CREATE INDEX idx_friendships_low ON friendships(user_id_low);
CREATE INDEX idx_friendships_high ON friendships(user_id_high);

CREATE VIEW friendship_edges AS
    SELECT user_id_low AS user_id, user_id_high AS friend_id, created_at FROM friendships
    UNION ALL
    SELECT user_id_high AS user_id, user_id_low AS friend_id, created_at FROM friendships;

CREATE TABLE user_blocks (
    block_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    blocker_id      INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    blocked_id      INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (blocker_id <> blocked_id),
    UNIQUE (blocker_id, blocked_id)
);

CREATE INDEX idx_user_blocks_blocked ON user_blocks(blocked_id);

CREATE TABLE follows (
    follow_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    follower_id     INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    followee_id     INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (follower_id <> followee_id),
    UNIQUE (follower_id, followee_id)
);

CREATE INDEX idx_follows_followee ON follows(followee_id);
CREATE INDEX idx_follows_follower ON follows(follower_id);

CREATE TABLE groups (
    group_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    description     TEXT,
    privacy         TEXT NOT NULL DEFAULT 'public'
                        CHECK (privacy IN ('public','private','secret')),
    cover_photo_url TEXT,
    creator_id      INTEGER NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_groups_creator ON groups(creator_id);

CREATE TABLE group_members (
    group_id        INTEGER NOT NULL REFERENCES groups(group_id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'member'
                        CHECK (role IN ('member','moderator','admin')),
    status          TEXT NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active','pending_approval','banned')),
    joined_at       TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX idx_group_members_user ON group_members(user_id, status);
CREATE INDEX idx_group_members_group_role ON group_members(group_id, role);

CREATE TABLE pages (
    page_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    category        TEXT,
    description     TEXT,
    profile_photo_url TEXT,
    cover_photo_url TEXT,
    website         TEXT,
    owner_id        INTEGER NOT NULL REFERENCES users(user_id) ON DELETE RESTRICT,
    is_verified     INTEGER NOT NULL DEFAULT 0 CHECK (is_verified IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_pages_owner ON pages(owner_id);
CREATE INDEX idx_pages_category ON pages(category);

CREATE TABLE page_admins (
    page_id         INTEGER NOT NULL REFERENCES pages(page_id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'editor'
                        CHECK (role IN ('admin','editor','analyst')),
    added_at        TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (page_id, user_id)
);

CREATE TABLE page_likes (
    page_id         INTEGER NOT NULL REFERENCES pages(page_id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    liked_at        TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (page_id, user_id)
);

CREATE INDEX idx_page_likes_user ON page_likes(user_id);

CREATE TABLE posts (
    post_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    author_id       INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    group_id        INTEGER REFERENCES groups(group_id) ON DELETE CASCADE,
    page_id         INTEGER REFERENCES pages(page_id) ON DELETE CASCADE,
    content         TEXT,
    media_url       TEXT,
    visibility      TEXT NOT NULL DEFAULT 'public'
                        CHECK (visibility IN ('public','friends','private')),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (NOT (group_id IS NOT NULL AND page_id IS NOT NULL))
);

CREATE INDEX idx_posts_author ON posts(author_id, created_at);
CREATE INDEX idx_posts_group ON posts(group_id, created_at);
CREATE INDEX idx_posts_page ON posts(page_id, created_at);

CREATE TABLE comments (
    comment_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id         INTEGER NOT NULL REFERENCES posts(post_id) ON DELETE CASCADE,
    author_id       INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    parent_comment_id INTEGER REFERENCES comments(comment_id) ON DELETE CASCADE,
    content         TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_comments_post ON comments(post_id);
CREATE INDEX idx_comments_parent ON comments(parent_comment_id);

CREATE TABLE reactions (
    reaction_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    post_id         INTEGER REFERENCES posts(post_id) ON DELETE CASCADE,
    comment_id      INTEGER REFERENCES comments(comment_id) ON DELETE CASCADE,
    reaction_type   TEXT NOT NULL DEFAULT 'like'
                        CHECK (reaction_type IN ('like','love','haha','wow','sad','angry')),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (
        (post_id IS NOT NULL AND comment_id IS NULL) OR
        (post_id IS NULL AND comment_id IS NOT NULL)
    ),
    UNIQUE (user_id, post_id, comment_id)
);

CREATE INDEX idx_reactions_post ON reactions(post_id);
CREATE INDEX idx_reactions_comment ON reactions(comment_id);
CREATE INDEX idx_reactions_user ON reactions(user_id);

CREATE TABLE friend_recommendations (
    user_id             INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    recommended_user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    mutual_friend_count INTEGER NOT NULL DEFAULT 0,
    mutual_group_count  INTEGER NOT NULL DEFAULT 0,
    score               REAL NOT NULL DEFAULT 0,
    generated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (user_id, recommended_user_id),
    CHECK (user_id <> recommended_user_id)
);

CREATE INDEX idx_friend_recs_user_score ON friend_recommendations(user_id, score DESC);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
FOR EACH ROW
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE user_id = NEW.user_id;
END;

CREATE TRIGGER trg_groups_updated_at
AFTER UPDATE ON groups
FOR EACH ROW
BEGIN
    UPDATE groups SET updated_at = datetime('now') WHERE group_id = NEW.group_id;
END;

CREATE TRIGGER trg_pages_updated_at
AFTER UPDATE ON pages
FOR EACH ROW
BEGIN
    UPDATE pages SET updated_at = datetime('now') WHERE page_id = NEW.page_id;
END;

CREATE TRIGGER trg_posts_updated_at
AFTER UPDATE ON posts
FOR EACH ROW
BEGIN
    UPDATE posts SET updated_at = datetime('now') WHERE post_id = NEW.post_id;
END;

CREATE TRIGGER trg_friend_request_accepted
AFTER UPDATE OF status ON friend_requests
FOR EACH ROW
WHEN NEW.status = 'accepted' AND OLD.status <> 'accepted'
BEGIN
    UPDATE friend_requests SET responded_at = datetime('now') WHERE request_id = NEW.request_id;

    INSERT OR IGNORE INTO friendships (user_id_low, user_id_high, source_request_id)
    VALUES (
        MIN(NEW.requester_id, NEW.addressee_id),
        MAX(NEW.requester_id, NEW.addressee_id),
        NEW.request_id
    );
END;

CREATE TRIGGER trg_friend_request_closed
AFTER UPDATE OF status ON friend_requests
FOR EACH ROW
WHEN NEW.status IN ('declined','cancelled') AND OLD.status <> NEW.status
BEGIN
    UPDATE friend_requests SET responded_at = datetime('now') WHERE request_id = NEW.request_id;
END;
`
