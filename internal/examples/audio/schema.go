package audio

const Schema = `
-- Minimal audio streaming service schema

PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id             INTEGER PRIMARY KEY,
    uuid                TEXT NOT NULL UNIQUE,
    email               TEXT NOT NULL UNIQUE,
    email_verified      INTEGER NOT NULL DEFAULT 0 CHECK (email_verified IN (0,1)),
    password_hash       TEXT,
    display_name        TEXT NOT NULL,
    country_code        TEXT NOT NULL,
    birthdate           TEXT,
    gender              TEXT,
    profile_image_url   TEXT,
    account_type        TEXT NOT NULL DEFAULT 'free'
                            CHECK (account_type IN ('free','premium','family','student','artist')),
    status               TEXT NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active','suspended','deactivated','deleted')),
    created_at          TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at          TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    last_login_at       TEXT
);

CREATE INDEX idx_users_country ON users(country_code);
CREATE INDEX idx_users_status  ON users(status);

CREATE TABLE user_oauth_accounts (
    oauth_id        INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    provider        TEXT NOT NULL CHECK (provider IN ('google','facebook','apple')),
    provider_uid    TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (provider, provider_uid)
);

CREATE TABLE devices (
    device_id       INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    device_uuid     TEXT NOT NULL UNIQUE,
    device_name     TEXT,
    device_type     TEXT NOT NULL CHECK (device_type IN ('mobile','desktop','web','tv','speaker','car','tablet')),
    os              TEXT,
    app_version     TEXT,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    last_seen_at    TEXT,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_devices_user ON devices(user_id);

CREATE TABLE sessions (
    session_id      INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    device_id       INTEGER REFERENCES devices(device_id) ON DELETE SET NULL,
    refresh_token   TEXT NOT NULL UNIQUE,
    ip_address      TEXT,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    expires_at      TEXT NOT NULL,
    revoked_at      TEXT
);

CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE plans (
    plan_id             INTEGER PRIMARY KEY,
    plan_code           TEXT NOT NULL UNIQUE,
    name                TEXT NOT NULL,
    price_cents         INTEGER NOT NULL DEFAULT 0,
    currency            TEXT NOT NULL DEFAULT 'USD',
    billing_interval    TEXT NOT NULL DEFAULT 'monthly' CHECK (billing_interval IN ('monthly','yearly','none')),
    max_family_members  INTEGER NOT NULL DEFAULT 1,
    ad_supported        INTEGER NOT NULL DEFAULT 1 CHECK (ad_supported IN (0,1)),
    offline_downloads   INTEGER NOT NULL DEFAULT 0 CHECK (offline_downloads IN (0,1)),
    max_bitrate_kbps    INTEGER NOT NULL DEFAULT 128,
    is_active           INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1))
);

CREATE TABLE subscriptions (
    subscription_id     INTEGER PRIMARY KEY,
    user_id             INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    plan_id             INTEGER NOT NULL REFERENCES plans(plan_id),
    family_owner_id     INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    status              TEXT NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active','trialing','past_due','canceled','expired')),
    start_date          TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    current_period_end  TEXT NOT NULL,
    cancel_at_period_end INTEGER NOT NULL DEFAULT 0 CHECK (cancel_at_period_end IN (0,1)),
    canceled_at         TEXT,
    created_at          TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);

CREATE TABLE payments (
    payment_id          INTEGER PRIMARY KEY,
    subscription_id     INTEGER NOT NULL REFERENCES subscriptions(subscription_id) ON DELETE CASCADE,
    amount_cents        INTEGER NOT NULL,
    currency            TEXT NOT NULL DEFAULT 'USD',
    payment_method      TEXT NOT NULL CHECK (payment_method IN ('card','paypal','apple_pay','google_pay','gift_card')),
    status              TEXT NOT NULL CHECK (status IN ('succeeded','failed','pending','refunded')),
    provider_txn_id     TEXT,
    created_at          TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_payments_subscription ON payments(subscription_id);

CREATE TABLE artists (
    artist_id       INTEGER PRIMARY KEY,
    uuid            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    bio             TEXT,
    country_code    TEXT,
    image_url       TEXT,
    verified        INTEGER NOT NULL DEFAULT 0 CHECK (verified IN (0,1)),
    monthly_listeners INTEGER NOT NULL DEFAULT 0,
    linked_user_id  INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_artists_name ON artists(name);

CREATE TABLE genres (
    genre_id        INTEGER PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    parent_genre_id INTEGER REFERENCES genres(genre_id) ON DELETE SET NULL
);

CREATE TABLE albums (
    album_id        INTEGER PRIMARY KEY,
    uuid            TEXT NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    album_type      TEXT NOT NULL DEFAULT 'album' CHECK (album_type IN ('album','single','ep','compilation')),
    release_date    TEXT NOT NULL,
    cover_image_url TEXT,
    label           TEXT,
    upc             TEXT,
    total_tracks    INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_albums_release_date ON albums(release_date);

CREATE TABLE album_artists (
    album_id        INTEGER NOT NULL REFERENCES albums(album_id) ON DELETE CASCADE,
    artist_id       INTEGER NOT NULL REFERENCES artists(artist_id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'primary' CHECK (role IN ('primary','featured')),
    PRIMARY KEY (album_id, artist_id)
);

CREATE TABLE tracks (
    track_id        INTEGER PRIMARY KEY,
    uuid            TEXT NOT NULL UNIQUE,
    title           TEXT NOT NULL,
    album_id        INTEGER REFERENCES albums(album_id) ON DELETE CASCADE,
    track_number    INTEGER,
    disc_number     INTEGER NOT NULL DEFAULT 1,
    duration_ms     INTEGER NOT NULL,
    isrc            TEXT,
    explicit        INTEGER NOT NULL DEFAULT 0 CHECK (explicit IN (0,1)),
    audio_file_url  TEXT NOT NULL,
    preview_url     TEXT,
    popularity      INTEGER NOT NULL DEFAULT 0,
    play_count      INTEGER NOT NULL DEFAULT 0,
    lyrics          TEXT,
    is_playable     INTEGER NOT NULL DEFAULT 1 CHECK (is_playable IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_tracks_album ON tracks(album_id);
CREATE INDEX idx_tracks_popularity ON tracks(popularity DESC);
CREATE INDEX idx_tracks_title ON tracks(title);

CREATE TABLE track_artists (
    track_id        INTEGER NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    artist_id       INTEGER NOT NULL REFERENCES artists(artist_id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'primary' CHECK (role IN ('primary','featured','producer','writer')),
    PRIMARY KEY (track_id, artist_id, role)
);

CREATE INDEX idx_track_artists_artist ON track_artists(artist_id);

CREATE TABLE track_genres (
    track_id        INTEGER NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    genre_id        INTEGER NOT NULL REFERENCES genres(genre_id) ON DELETE CASCADE,
    PRIMARY KEY (track_id, genre_id)
);

CREATE TABLE playlists (
    playlist_id     INTEGER PRIMARY KEY,
    uuid            TEXT NOT NULL UNIQUE,
    owner_id        INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    description     TEXT,
    cover_image_url TEXT,
    is_public       INTEGER NOT NULL DEFAULT 1 CHECK (is_public IN (0,1)),
    is_collaborative INTEGER NOT NULL DEFAULT 0 CHECK (is_collaborative IN (0,1)),
    playlist_type   TEXT NOT NULL DEFAULT 'user' CHECK (playlist_type IN ('user','editorial','algorithmic')),
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_playlists_owner ON playlists(owner_id);

CREATE TABLE playlist_tracks (
    playlist_id     INTEGER NOT NULL REFERENCES playlists(playlist_id) ON DELETE CASCADE,
    track_id        INTEGER NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    position        INTEGER NOT NULL,
    added_by_user_id INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    added_at        TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (playlist_id, track_id, position)
);

CREATE INDEX idx_playlist_tracks_track ON playlist_tracks(track_id);

CREATE TABLE playlist_collaborators (
    playlist_id     INTEGER NOT NULL REFERENCES playlists(playlist_id) ON DELETE CASCADE,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    permission      TEXT NOT NULL DEFAULT 'edit' CHECK (permission IN ('edit','view')),
    added_at        TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (playlist_id, user_id)
);

CREATE TABLE liked_tracks (
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    track_id        INTEGER NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    liked_at        TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (user_id, track_id)
);

CREATE TABLE saved_albums (
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    album_id        INTEGER NOT NULL REFERENCES albums(album_id) ON DELETE CASCADE,
    saved_at        TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (user_id, album_id)
);

CREATE TABLE saved_playlists (
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    playlist_id     INTEGER NOT NULL REFERENCES playlists(playlist_id) ON DELETE CASCADE,
    saved_at        TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (user_id, playlist_id)
);

CREATE TABLE artist_follows (
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    artist_id       INTEGER NOT NULL REFERENCES artists(artist_id) ON DELETE CASCADE,
    followed_at     TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (user_id, artist_id)
);

CREATE TABLE user_follows (
    follower_id     INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    followee_id     INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    followed_at     TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (follower_id, followee_id),
    CHECK (follower_id != followee_id)
);

CREATE TABLE playback_history (
    playback_id     INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    track_id        INTEGER NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    device_id       INTEGER REFERENCES devices(device_id) ON DELETE SET NULL,
    context_type    TEXT CHECK (context_type IN ('playlist','album','artist','search','radio','queue')),
    context_id      INTEGER,
    ms_played       INTEGER NOT NULL DEFAULT 0,
    shuffle         INTEGER NOT NULL DEFAULT 0 CHECK (shuffle IN (0,1)),
    skipped         INTEGER NOT NULL DEFAULT 0 CHECK (skipped IN (0,1)),
    played_at       TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_playback_user_time ON playback_history(user_id, played_at DESC);
CREATE INDEX idx_playback_track ON playback_history(track_id);

CREATE TABLE now_playing (
    user_id         INTEGER PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    track_id        INTEGER REFERENCES tracks(track_id) ON DELETE SET NULL,
    device_id       INTEGER REFERENCES devices(device_id) ON DELETE SET NULL,
    position_ms     INTEGER NOT NULL DEFAULT 0,
    is_playing      INTEGER NOT NULL DEFAULT 0 CHECK (is_playing IN (0,1)),
    repeat_mode     TEXT NOT NULL DEFAULT 'off' CHECK (repeat_mode IN ('off','track','context')),
    shuffle         INTEGER NOT NULL DEFAULT 0 CHECK (shuffle IN (0,1)),
    updated_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE downloads (
    download_id     INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    device_id       INTEGER NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
    track_id        INTEGER NOT NULL REFERENCES tracks(track_id) ON DELETE CASCADE,
    downloaded_at   TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (device_id, track_id)
);

CREATE TABLE search_history (
    search_id       INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    query           TEXT NOT NULL,
    searched_at     TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_search_history_user ON search_history(user_id, searched_at DESC);

CREATE TABLE radio_seeds (
    radio_id        INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    seed_type       TEXT NOT NULL CHECK (seed_type IN ('track','artist','genre')),
    seed_id         INTEGER NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = STRFTIME('%Y-%m-%dT%H:%M:%fZ','now') WHERE user_id = NEW.user_id;
END;

CREATE TRIGGER trg_playlists_updated_at
AFTER UPDATE ON playlists
BEGIN
    UPDATE playlists SET updated_at = STRFTIME('%Y-%m-%dT%H:%M:%fZ','now') WHERE playlist_id = NEW.playlist_id;
END;

CREATE TRIGGER trg_track_play_count
AFTER INSERT ON playback_history
WHEN NEW.ms_played > 30000
BEGIN
    UPDATE tracks SET play_count = play_count + 1 WHERE track_id = NEW.track_id;
END;

CREATE TRIGGER trg_album_total_tracks_insert
AFTER INSERT ON tracks
WHEN NEW.album_id IS NOT NULL
BEGIN
    UPDATE albums SET total_tracks = (
        SELECT COUNT(*) FROM tracks WHERE album_id = NEW.album_id
    ) WHERE album_id = NEW.album_id;
END;

CREATE TRIGGER trg_album_total_tracks_delete
AFTER DELETE ON tracks
WHEN OLD.album_id IS NOT NULL
BEGIN
    UPDATE albums SET total_tracks = (
        SELECT COUNT(*) FROM tracks WHERE album_id = OLD.album_id
    ) WHERE album_id = OLD.album_id;
END;

INSERT INTO plans (plan_code, name, price_cents, currency, billing_interval, max_family_members, ad_supported, offline_downloads, max_bitrate_kbps)
VALUES
    ('free',               'Free',               0,    'USD', 'none',    1, 1, 0, 128),
    ('premium_individual', 'Premium Individual',  1099, 'USD', 'monthly',1, 0, 1, 320),
    ('premium_duo',        'Premium Duo',         1399, 'USD', 'monthly',2, 0, 1, 320),
    ('premium_family',     'Premium Family',      1699, 'USD', 'monthly',6, 0, 1, 320),
    ('premium_student',    'Premium Student',     599,  'USD', 'monthly',1, 0, 1, 320);
`
