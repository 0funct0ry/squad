package streaming

const Schema = `-- Movie streaming platform schema
PRAGMA foreign_keys = ON;

CREATE TABLE regions (
    region_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    region_code     TEXT NOT NULL UNIQUE,
    region_name     TEXT NOT NULL,
    currency_code   TEXT NOT NULL DEFAULT 'USD'
);

CREATE TABLE genres (
    genre_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    genre_name      TEXT NOT NULL UNIQUE
);

CREATE TABLE languages (
    language_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    language_code   TEXT NOT NULL UNIQUE,
    language_name   TEXT NOT NULL
);

CREATE TABLE maturity_ratings (
    rating_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    region_id       INTEGER NOT NULL REFERENCES regions(region_id),
    code            TEXT NOT NULL,
    description     TEXT,
    UNIQUE (region_id, code)
);

CREATE TABLE people (
    person_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name       TEXT NOT NULL,
    date_of_birth   DATE,
    country         TEXT,
    bio             TEXT
);

CREATE TABLE studios (
    studio_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    studio_name     TEXT NOT NULL UNIQUE,
    contact_email   TEXT,
    country         TEXT
);

CREATE TABLE titles (
    title_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    title_type      TEXT NOT NULL CHECK (title_type IN ('MOVIE', 'SERIES', 'DOCUMENTARY', 'STANDUP', 'SHORT')),
    original_title  TEXT NOT NULL,
    display_title   TEXT NOT NULL,
    synopsis        TEXT,
    release_year    INTEGER,
    runtime_minutes INTEGER,
    original_language_id INTEGER REFERENCES languages(language_id),
    is_netflix_original INTEGER NOT NULL DEFAULT 0 CHECK (is_netflix_original IN (0,1)),
    status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','ARCHIVED','COMING_SOON','REMOVED')),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE seasons (
    season_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id        INTEGER NOT NULL REFERENCES titles(title_id) ON DELETE CASCADE,
    season_number   INTEGER NOT NULL,
    season_name     TEXT,
    release_year    INTEGER,
    UNIQUE (title_id, season_number)
);

CREATE TABLE episodes (
    episode_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    season_id       INTEGER NOT NULL REFERENCES seasons(season_id) ON DELETE CASCADE,
    episode_number  INTEGER NOT NULL,
    episode_title   TEXT NOT NULL,
    synopsis        TEXT,
    runtime_minutes INTEGER NOT NULL,
    air_date        DATE,
    UNIQUE (season_id, episode_number)
);

CREATE TABLE title_genres (
    title_id        INTEGER NOT NULL REFERENCES titles(title_id) ON DELETE CASCADE,
    genre_id        INTEGER NOT NULL REFERENCES genres(genre_id) ON DELETE CASCADE,
    PRIMARY KEY (title_id, genre_id)
);

CREATE TABLE title_cast_crew (
    title_id        INTEGER NOT NULL REFERENCES titles(title_id) ON DELETE CASCADE,
    person_id       INTEGER NOT NULL REFERENCES people(person_id) ON DELETE CASCADE,
    role_type       TEXT NOT NULL CHECK (role_type IN ('ACTOR','DIRECTOR','WRITER','PRODUCER','CREATOR')),
    character_name  TEXT,
    billing_order   INTEGER,
    PRIMARY KEY (title_id, person_id, role_type)
);

CREATE TABLE title_maturity_ratings (
    title_id        INTEGER NOT NULL REFERENCES titles(title_id) ON DELETE CASCADE,
    rating_id       INTEGER NOT NULL REFERENCES maturity_ratings(rating_id),
    PRIMARY KEY (title_id, rating_id)
);

CREATE TABLE media_assets (
    asset_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id        INTEGER REFERENCES titles(title_id) ON DELETE CASCADE,
    episode_id      INTEGER REFERENCES episodes(episode_id) ON DELETE CASCADE,
    asset_type      TEXT NOT NULL CHECK (asset_type IN ('VIDEO_MASTER','TRAILER','POSTER','THUMBNAIL','SUBTITLE','AUDIO_TRACK')),
    storage_url     TEXT NOT NULL,
    resolution      TEXT,
    language_id     INTEGER REFERENCES languages(language_id),
    file_size_mb    INTEGER,
    CHECK ( (title_id IS NOT NULL AND episode_id IS NULL)
         OR (title_id IS NULL AND episode_id IS NOT NULL) )
);

CREATE TABLE licensing_deals (
    deal_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id            INTEGER NOT NULL REFERENCES titles(title_id) ON DELETE CASCADE,
    studio_id           INTEGER NOT NULL REFERENCES studios(studio_id),
    deal_type           TEXT NOT NULL CHECK (deal_type IN ('LICENSED','ORIGINAL_PRODUCTION','CO_PRODUCTION','ACQUISITION')),
    total_cost_usd      NUMERIC(14,2),
    contract_signed_date DATE NOT NULL,
    notes               TEXT
);

CREATE TABLE licensing_rights (
    right_id            INTEGER PRIMARY KEY AUTOINCREMENT,
    deal_id             INTEGER NOT NULL REFERENCES licensing_deals(deal_id) ON DELETE CASCADE,
    region_id           INTEGER NOT NULL REFERENCES regions(region_id),
    window_start_date   DATE NOT NULL,
    window_end_date     DATE NOT NULL,
    is_exclusive         INTEGER NOT NULL DEFAULT 0 CHECK (is_exclusive IN (0,1)),
    UNIQUE (deal_id, region_id, window_start_date),
    CHECK (window_end_date > window_start_date)
);

CREATE TABLE plans (
    plan_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_name       TEXT NOT NULL UNIQUE,
    max_profiles    INTEGER NOT NULL DEFAULT 1,
    max_simultaneous_streams INTEGER NOT NULL DEFAULT 1,
    max_resolution  TEXT NOT NULL DEFAULT 'HD',
    monthly_price_usd NUMERIC(6,2) NOT NULL
);

CREATE TABLE accounts (
    account_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    country_region_id INTEGER NOT NULL REFERENCES regions(region_id),
    signup_date     TEXT NOT NULL DEFAULT (datetime('now')),
    account_status  TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (account_status IN ('ACTIVE','SUSPENDED','CANCELLED'))
);

CREATE TABLE subscriptions (
    subscription_id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      INTEGER NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
    plan_id         INTEGER NOT NULL REFERENCES plans(plan_id),
    start_date      DATE NOT NULL,
    end_date        DATE,
    status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','PAUSED','CANCELLED','EXPIRED'))
);

CREATE TABLE payments (
    payment_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(subscription_id) ON DELETE CASCADE,
    amount_usd      NUMERIC(8,2) NOT NULL,
    payment_date    TEXT NOT NULL DEFAULT (datetime('now')),
    payment_method  TEXT NOT NULL CHECK (payment_method IN ('CREDIT_CARD','DEBIT_CARD','PAYPAL','GIFT_CARD','UPI')),
    payment_status  TEXT NOT NULL DEFAULT 'SUCCESS' CHECK (payment_status IN ('SUCCESS','FAILED','REFUNDED'))
);

CREATE TABLE profiles (
    profile_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      INTEGER NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
    profile_name    TEXT NOT NULL,
    avatar_url      TEXT,
    is_kids_profile INTEGER NOT NULL DEFAULT 0 CHECK (is_kids_profile IN (0,1)),
    preferred_language_id INTEGER REFERENCES languages(language_id),
    autoplay_enabled INTEGER NOT NULL DEFAULT 1 CHECK (autoplay_enabled IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (account_id, profile_name)
);

CREATE TABLE devices (
    device_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id      INTEGER NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
    device_type     TEXT NOT NULL CHECK (device_type IN ('SMART_TV','MOBILE','TABLET','WEB','GAME_CONSOLE','STREAMING_STICK')),
    device_name     TEXT,
    last_seen_at    TEXT
);

CREATE TABLE viewing_history (
    view_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id      INTEGER NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    title_id        INTEGER NOT NULL REFERENCES titles(title_id),
    episode_id      INTEGER REFERENCES episodes(episode_id),
    device_id       INTEGER REFERENCES devices(device_id),
    started_at      TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at        TEXT,
    playback_position_seconds INTEGER NOT NULL DEFAULT 0,
    total_duration_seconds    INTEGER NOT NULL,
    completed       INTEGER NOT NULL DEFAULT 0 CHECK (completed IN (0,1)),
    region_id       INTEGER REFERENCES regions(region_id)
);

CREATE INDEX idx_viewing_history_profile ON viewing_history(profile_id, started_at);
CREATE INDEX idx_viewing_history_title ON viewing_history(title_id);

CREATE TABLE ratings (
    rating_event_id INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id      INTEGER NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    title_id        INTEGER NOT NULL REFERENCES titles(title_id) ON DELETE CASCADE,
    thumbs_rating   INTEGER NOT NULL CHECK (thumbs_rating IN (-1, 1, 2)),
    rated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (profile_id, title_id)
);

CREATE TABLE watchlist (
    watchlist_id    INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id      INTEGER NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    title_id        INTEGER NOT NULL REFERENCES titles(title_id) ON DELETE CASCADE,
    added_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (profile_id, title_id)
);

CREATE TABLE recommendation_models (
    model_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    model_name      TEXT NOT NULL,
    model_version   TEXT NOT NULL,
    deployed_at     TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (model_name, model_version)
);

CREATE TABLE recommendations (
    recommendation_id INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id      INTEGER NOT NULL REFERENCES profiles(profile_id) ON DELETE CASCADE,
    title_id        INTEGER NOT NULL REFERENCES titles(title_id) ON DELETE CASCADE,
    model_id        INTEGER NOT NULL REFERENCES recommendation_models(model_id),
    score           REAL NOT NULL,
    rank_position   INTEGER,
    generated_at    TEXT NOT NULL DEFAULT (datetime('now')),
    context_tag     TEXT,
    UNIQUE (profile_id, title_id, model_id, generated_at)
);

CREATE INDEX idx_recommendations_profile ON recommendations(profile_id, generated_at);

CREATE TABLE recommendation_impressions (
    impression_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    recommendation_id INTEGER NOT NULL REFERENCES recommendations(recommendation_id) ON DELETE CASCADE,
    shown_at        TEXT NOT NULL DEFAULT (datetime('now')),
    was_clicked     INTEGER NOT NULL DEFAULT 0 CHECK (was_clicked IN (0,1))
);

CREATE VIEW v_active_catalog_by_region AS
SELECT t.title_id, t.display_title, t.title_type, r.region_code,
       lr.window_start_date, lr.window_end_date, lr.is_exclusive
FROM titles t
JOIN licensing_deals ld ON ld.title_id = t.title_id
JOIN licensing_rights lr ON lr.deal_id = ld.deal_id
JOIN regions r ON r.region_id = lr.region_id
WHERE date('now') BETWEEN lr.window_start_date AND lr.window_end_date
  AND t.status = 'ACTIVE';

CREATE VIEW v_continue_watching AS
SELECT vh.profile_id, vh.title_id, vh.episode_id,
       vh.playback_position_seconds, vh.total_duration_seconds,
       vh.started_at
FROM viewing_history vh
WHERE vh.completed = 0
  AND vh.playback_position_seconds > 0;
`
