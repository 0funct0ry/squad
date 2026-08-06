package chat

const Schema = `-- Chat Platform Schema

PRAGMA foreign_keys = ON;

CREATE TABLE users (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    username        TEXT NOT NULL UNIQUE,
    display_name    TEXT,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    avatar_url      TEXT,
    status          TEXT NOT NULL DEFAULT 'offline'
                        CHECK (status IN ('online','offline','away','dnd')),
    status_message  TEXT,
    is_bot          INTEGER NOT NULL DEFAULT 0 CHECK (is_bot IN (0,1)),
    is_deleted      INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE workspaces (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    description     TEXT,
    icon_url        TEXT,
    owner_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    is_deleted      INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_workspaces_owner ON workspaces(owner_id);

CREATE TABLE roles (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id    INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    is_system_role  INTEGER NOT NULL DEFAULT 0 CHECK (is_system_role IN (0,1)),
    color_hex       TEXT,
    "position"      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (workspace_id, name)
);

CREATE INDEX idx_roles_workspace ON roles(workspace_id);

CREATE TABLE permissions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    code            TEXT NOT NULL UNIQUE,
    description     TEXT
);

CREATE TABLE role_permissions (
    role_id         INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id   INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE workspace_memberships (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id    INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nickname        TEXT,
    joined_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    UNIQUE (workspace_id, user_id)
);

CREATE INDEX idx_ws_memberships_user ON workspace_memberships(user_id);
CREATE INDEX idx_ws_memberships_ws ON workspace_memberships(workspace_id);

CREATE TABLE membership_roles (
    membership_id   INTEGER NOT NULL REFERENCES workspace_memberships(id) ON DELETE CASCADE,
    role_id         INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (membership_id, role_id)
);

CREATE TABLE channels (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id    INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    parent_id       INTEGER REFERENCES channels(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    topic           TEXT,
    type            TEXT NOT NULL DEFAULT 'text'
                        CHECK (type IN ('text','voice','announcement','category')),
    is_private      INTEGER NOT NULL DEFAULT 0 CHECK (is_private IN (0,1)),
    is_archived     INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0,1)),
    created_by      INTEGER REFERENCES users(id) ON DELETE SET NULL,
    "position"      INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (workspace_id, name, parent_id)
);

CREATE INDEX idx_channels_workspace ON channels(workspace_id);
CREATE INDEX idx_channels_parent ON channels(parent_id);

CREATE TABLE channel_memberships (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id      INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    last_read_message_id INTEGER,
    is_muted        INTEGER NOT NULL DEFAULT 0 CHECK (is_muted IN (0,1)),
    UNIQUE (channel_id, user_id)
);

CREATE INDEX idx_channel_memberships_user ON channel_memberships(user_id);
CREATE INDEX idx_channel_memberships_channel ON channel_memberships(channel_id);

CREATE TABLE channel_permission_overwrites (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id      INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    role_id         INTEGER REFERENCES roles(id) ON DELETE CASCADE,
    user_id         INTEGER REFERENCES users(id) ON DELETE CASCADE,
    permission_id   INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    allow           INTEGER NOT NULL DEFAULT 1 CHECK (allow IN (0,1)),
    CHECK (
        (role_id IS NOT NULL AND user_id IS NULL) OR
        (role_id IS NULL AND user_id IS NOT NULL)
    )
);

CREATE INDEX idx_cpo_channel ON channel_permission_overwrites(channel_id);

CREATE TABLE messages (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id          INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id             INTEGER REFERENCES users(id) ON DELETE SET NULL,
    thread_id           INTEGER REFERENCES threads(id) ON DELETE SET NULL,
    parent_message_id   INTEGER REFERENCES messages(id) ON DELETE SET NULL, -- reply-to
    content             TEXT,
    is_edited           INTEGER NOT NULL DEFAULT 0 CHECK (is_edited IN (0,1)),
    is_deleted          INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1)),
    is_pinned           INTEGER NOT NULL DEFAULT 0 CHECK (is_pinned IN (0,1)),
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_messages_channel_created ON messages(channel_id, created_at);
CREATE INDEX idx_messages_user ON messages(user_id);
CREATE INDEX idx_messages_thread ON messages(thread_id);
CREATE INDEX idx_messages_parent ON messages(parent_message_id);

CREATE TABLE threads (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id          INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    root_message_id     INTEGER NOT NULL UNIQUE REFERENCES messages(id) ON DELETE CASCADE,
    reply_count         INTEGER NOT NULL DEFAULT 0,
    last_reply_at       TEXT,
    is_archived         INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0,1)),
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_threads_channel ON threads(channel_id);

CREATE TABLE attachments (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id      INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_name       TEXT NOT NULL,
    file_url        TEXT NOT NULL,
    mime_type       TEXT,
    file_size_bytes INTEGER,
    width           INTEGER,
    height          INTEGER,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_attachments_message ON attachments(message_id);

CREATE TABLE emojis (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id    INTEGER REFERENCES workspaces(id) ON DELETE CASCADE,
    code            TEXT NOT NULL,
    image_url       TEXT,
    unicode_char    TEXT,
    created_by      INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (workspace_id, code)
);

CREATE TABLE reactions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id      INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji_id        INTEGER NOT NULL REFERENCES emojis(id) ON DELETE CASCADE,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (message_id, user_id, emoji_id)
);

CREATE INDEX idx_reactions_message ON reactions(message_id);

CREATE TABLE dm_conversations (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id    INTEGER REFERENCES workspaces(id) ON DELETE CASCADE,
    is_group        INTEGER NOT NULL DEFAULT 0 CHECK (is_group IN (0,1)),
    name            TEXT,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE dm_participants (
    dm_conversation_id INTEGER NOT NULL REFERENCES dm_conversations(id) ON DELETE CASCADE,
    user_id             INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (dm_conversation_id, user_id)
);

CREATE TABLE dm_messages (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    dm_conversation_id  INTEGER NOT NULL REFERENCES dm_conversations(id) ON DELETE CASCADE,
    user_id             INTEGER REFERENCES users(id) ON DELETE SET NULL,
    content             TEXT,
    is_edited           INTEGER NOT NULL DEFAULT 0 CHECK (is_edited IN (0,1)),
    is_deleted          INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1)),
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_dm_messages_conv_created ON dm_messages(dm_conversation_id, created_at);

CREATE TABLE invites (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id    INTEGER NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    code            TEXT NOT NULL UNIQUE,
    created_by      INTEGER REFERENCES users(id) ON DELETE SET NULL,
    max_uses        INTEGER,
    use_count       INTEGER NOT NULL DEFAULT 0,
    expires_at      TEXT,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_invites_workspace ON invites(workspace_id);

INSERT INTO permissions (code, description) VALUES
    ('MANAGE_WORKSPACE',   'Edit workspace settings'),
    ('MANAGE_ROLES',       'Create/edit/delete roles'),
    ('MANAGE_CHANNELS',    'Create/edit/delete channels'),
    ('MANAGE_MEMBERS',     'Kick/ban/invite members'),
    ('MANAGE_MESSAGES',    'Delete/pin others'' messages'),
    ('SEND_MESSAGES',      'Post messages in a channel'),
    ('READ_MESSAGES',      'View channel message history'),
    ('ADD_REACTIONS',      'React to messages with emoji'),
    ('ATTACH_FILES',       'Upload file attachments'),
    ('CREATE_THREADS',     'Start threaded replies');

INSERT INTO emojis (workspace_id, code, unicode_char) VALUES
    (NULL, 'thumbsup', '👍'),
    (NULL, 'heart',    '❤️'),
    (NULL, 'joy',      '😂'),
    (NULL, 'eyes',     '👀');
`
