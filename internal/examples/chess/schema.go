package chess

const Schema = `-- Online Chess Platform Schema

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

CREATE TABLE users (
    user_id         INTEGER PRIMARY KEY,
    username        TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    display_name    TEXT,
    country_code    TEXT,
    avatar_url      TEXT,
    bio             TEXT,
    account_status  TEXT NOT NULL DEFAULT 'active'
                        CHECK (account_status IN ('active','suspended','banned','deactivated')),
    is_verified     INTEGER NOT NULL DEFAULT 0 CHECK (is_verified IN (0,1)),
    is_admin        INTEGER NOT NULL DEFAULT 0 CHECK (is_admin IN (0,1)),
    timezone        TEXT DEFAULT 'UTC',
    locale          TEXT DEFAULT 'en',
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    last_login_at   TEXT,
    is_deleted      INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email    ON users(email);
CREATE INDEX idx_users_country  ON users(country_code);

CREATE TABLE user_auth_providers (
    auth_provider_id  INTEGER PRIMARY KEY,
    user_id           INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    provider          TEXT NOT NULL CHECK (provider IN ('google','apple','facebook','github','lichess','discord')),
    provider_user_id  TEXT NOT NULL,
    linked_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (provider, provider_user_id)
);

CREATE TABLE user_sessions (
    session_id  TEXT PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    ip_address  TEXT,
    user_agent  TEXT,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    expires_at  TEXT NOT NULL,
    revoked_at  TEXT
);

CREATE INDEX idx_sessions_user ON user_sessions(user_id);

CREATE TABLE players (
    player_id        INTEGER PRIMARY KEY,
    user_id          INTEGER NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
    fide_id          TEXT UNIQUE,
    fide_title       TEXT CHECK (fide_title IN ('GM','IM','FM','CM','WGM','WIM','WFM','WCM',NULL)),
    is_online_title  INTEGER NOT NULL DEFAULT 0 CHECK (is_online_title IN (0,1)),
    date_of_birth    DATE,
    joined_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_bot           INTEGER NOT NULL DEFAULT 0 CHECK (is_bot IN (0,1)),
    patron_since     TEXT,
    is_deleted       INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE INDEX idx_players_fide ON players(fide_id);

CREATE TABLE game_variants (
    variant_id   INTEGER PRIMARY KEY,
    code         TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    description  TEXT
);

CREATE TABLE ratings (
    rating_id       INTEGER PRIMARY KEY,
    player_id       INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    variant_id      INTEGER NOT NULL REFERENCES game_variants(variant_id),
    rating          INTEGER NOT NULL DEFAULT 1500,
    rd              REAL NOT NULL DEFAULT 350.0,
    volatility      REAL NOT NULL DEFAULT 0.06,
    games_played    INTEGER NOT NULL DEFAULT 0,
    is_provisional  INTEGER NOT NULL DEFAULT 1 CHECK (is_provisional IN (0,1)),
    peak_rating     INTEGER,
    peak_rating_at  TEXT,
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (player_id, variant_id)
);

CREATE INDEX idx_ratings_player          ON ratings(player_id);
CREATE INDEX idx_ratings_variant_rating  ON ratings(variant_id, rating DESC);

CREATE TABLE rating_history (
    rating_history_id  INTEGER PRIMARY KEY,
    player_id          INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    variant_id         INTEGER NOT NULL REFERENCES game_variants(variant_id),
    game_id            INTEGER REFERENCES games(game_id) ON DELETE SET NULL,
    rating_before      INTEGER NOT NULL,
    rating_after       INTEGER NOT NULL,
    rd_before          REAL,
    rd_after           REAL,
    delta              INTEGER NOT NULL,
    recorded_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_rating_history_player_variant ON rating_history(player_id, variant_id, recorded_at);

CREATE TABLE player_stats (
    stat_id               INTEGER PRIMARY KEY,
    player_id             INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    variant_id            INTEGER NOT NULL REFERENCES game_variants(variant_id),
    wins                  INTEGER NOT NULL DEFAULT 0,
    losses                INTEGER NOT NULL DEFAULT 0,
    draws                 INTEGER NOT NULL DEFAULT 0,
    current_win_streak    INTEGER NOT NULL DEFAULT 0,
    best_win_streak       INTEGER NOT NULL DEFAULT 0,
    total_time_played_s   INTEGER NOT NULL DEFAULT 0,
    puzzles_solved        INTEGER NOT NULL DEFAULT 0,
    tournaments_played    INTEGER NOT NULL DEFAULT 0,
    tournaments_won       INTEGER NOT NULL DEFAULT 0,
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (player_id, variant_id)
);

CREATE TABLE clubs (
    club_id            INTEGER PRIMARY KEY,
    name               TEXT NOT NULL UNIQUE,
    slug               TEXT NOT NULL UNIQUE,
    description        TEXT,
    logo_url           TEXT,
    founder_player_id  INTEGER REFERENCES players(player_id) ON DELETE SET NULL,
    is_private         INTEGER NOT NULL DEFAULT 0 CHECK (is_private IN (0,1)),
    member_count       INTEGER NOT NULL DEFAULT 0,
    created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_deleted         INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE INDEX idx_clubs_slug ON clubs(slug);

CREATE TABLE club_members (
    club_member_id  INTEGER PRIMARY KEY,
    club_id         INTEGER NOT NULL REFERENCES clubs(club_id) ON DELETE CASCADE,
    player_id       INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member')),
    joined_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (club_id, player_id)
);

CREATE INDEX idx_club_members_club    ON club_members(club_id);
CREATE INDEX idx_club_members_player  ON club_members(player_id);

CREATE TABLE teams (
    team_id            INTEGER PRIMARY KEY,
    club_id            INTEGER REFERENCES clubs(club_id) ON DELETE SET NULL,
    name               TEXT NOT NULL,
    slug               TEXT NOT NULL UNIQUE,
    captain_player_id  INTEGER REFERENCES players(player_id) ON DELETE SET NULL,
    created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_deleted         INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE TABLE team_members (
    team_member_id  INTEGER PRIMARY KEY,
    team_id         INTEGER NOT NULL REFERENCES teams(team_id) ON DELETE CASCADE,
    player_id       INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    board_number    INTEGER,
    joined_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (team_id, player_id)
);

CREATE TABLE team_matches (
    team_match_id   INTEGER PRIMARY KEY,
    home_team_id    INTEGER NOT NULL REFERENCES teams(team_id),
    away_team_id    INTEGER NOT NULL REFERENCES teams(team_id),
    tournament_id   INTEGER REFERENCES tournaments(tournament_id) ON DELETE SET NULL,
    scheduled_at    TEXT,
    home_score      REAL NOT NULL DEFAULT 0,
    away_score      REAL NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'scheduled'
                        CHECK (status IN ('scheduled','ongoing','completed','cancelled')),
    CHECK (home_team_id <> away_team_id)
);

CREATE TABLE team_match_boards (
    team_match_board_id  INTEGER PRIMARY KEY,
    team_match_id        INTEGER NOT NULL REFERENCES team_matches(team_match_id) ON DELETE CASCADE,
    board_number         INTEGER NOT NULL,
    game_id              INTEGER REFERENCES games(game_id) ON DELETE SET NULL,
    home_player_id       INTEGER REFERENCES players(player_id),
    away_player_id       INTEGER REFERENCES players(player_id),
    UNIQUE (team_match_id, board_number)
);

CREATE TABLE streamers (
    streamer_id        INTEGER PRIMARY KEY,
    player_id          INTEGER NOT NULL UNIQUE REFERENCES players(player_id) ON DELETE CASCADE,
    channel_name       TEXT NOT NULL,
    platform           TEXT NOT NULL CHECK (platform IN ('twitch','youtube','kick','other')),
    channel_url        TEXT NOT NULL,
    is_currently_live  INTEGER NOT NULL DEFAULT 0 CHECK (is_currently_live IN (0,1)),
    follower_count     INTEGER NOT NULL DEFAULT 0,
    approved_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_deleted         INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE TABLE stream_sessions (
    stream_session_id  INTEGER PRIMARY KEY,
    streamer_id        INTEGER NOT NULL REFERENCES streamers(streamer_id) ON DELETE CASCADE,
    title              TEXT,
    started_at         TEXT NOT NULL,
    ended_at           TEXT,
    peak_viewers       INTEGER,
    linked_game_id     INTEGER REFERENCES games(game_id) ON DELETE SET NULL
);

CREATE INDEX idx_stream_sessions_streamer ON stream_sessions(streamer_id);

CREATE TABLE content_creators (
    content_creator_id  INTEGER PRIMARY KEY,
    player_id           INTEGER NOT NULL UNIQUE REFERENCES players(player_id) ON DELETE CASCADE,
    creator_name        TEXT NOT NULL,
    bio                 TEXT,
    website_url         TEXT,
    is_verified         INTEGER NOT NULL DEFAULT 0 CHECK (is_verified IN (0,1)),
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE matches (
    match_id          INTEGER PRIMARY KEY,
    match_type        TEXT NOT NULL DEFAULT 'casual'
                          CHECK (match_type IN ('casual','rated','tournament','arena','challenge','simul')),
    player_white_id   INTEGER REFERENCES players(player_id),
    player_black_id   INTEGER REFERENCES players(player_id),
    variant_id        INTEGER NOT NULL REFERENCES game_variants(variant_id),
    tournament_id     INTEGER REFERENCES tournaments(tournament_id) ON DELETE SET NULL,
    arena_id          INTEGER REFERENCES arenas(arena_id) ON DELETE SET NULL,
    best_of           INTEGER NOT NULL DEFAULT 1,
    status            TEXT NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending','ongoing','completed','aborted')),
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE games (
    game_id                     INTEGER PRIMARY KEY,
    match_id                    INTEGER REFERENCES matches(match_id) ON DELETE SET NULL,
    variant_id                  INTEGER NOT NULL REFERENCES game_variants(variant_id),
    white_player_id             INTEGER REFERENCES players(player_id),
    black_player_id             INTEGER REFERENCES players(player_id),
    white_rating_at_start       INTEGER,
    black_rating_at_start       INTEGER,
    is_rated                    INTEGER NOT NULL DEFAULT 1 CHECK (is_rated IN (0,1)),
    time_control_initial_s      INTEGER NOT NULL,
    time_control_increment_s    INTEGER NOT NULL DEFAULT 0,
    starting_fen                TEXT NOT NULL DEFAULT 'rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1',
    final_fen                   TEXT,
    pgn                         TEXT,
    result                      TEXT CHECK (result IN ('1-0','0-1','1/2-1/2','*')),
    termination                 TEXT CHECK (termination IN
                                    ('checkmate','resignation','timeout','draw_agreement',
                                     'stalemate','insufficient_material','threefold_repetition',
                                     'fifty_move_rule','abandoned','aborted','rules_infraction')),
    winner_player_id            INTEGER REFERENCES players(player_id),
    opening_id                  INTEGER REFERENCES openings(opening_id) ON DELETE SET NULL,
    started_at                  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    ended_at                    TEXT,
    ply_count                   INTEGER NOT NULL DEFAULT 0,
    is_deleted                  INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE INDEX idx_games_white    ON games(white_player_id);
CREATE INDEX idx_games_black    ON games(black_player_id);
CREATE INDEX idx_games_match    ON games(match_id);
CREATE INDEX idx_games_opening  ON games(opening_id);
CREATE INDEX idx_games_started  ON games(started_at);
CREATE INDEX idx_games_variant  ON games(variant_id);

CREATE TABLE moves (
    move_id             INTEGER PRIMARY KEY,
    game_id             INTEGER NOT NULL REFERENCES games(game_id) ON DELETE CASCADE,
    ply_number          INTEGER NOT NULL,
    move_san            TEXT NOT NULL,
    move_uci            TEXT,
    fen_after           TEXT,
    clock_remaining_s   REAL,
    move_time_ms        INTEGER,
    is_check            INTEGER NOT NULL DEFAULT 0 CHECK (is_check IN (0,1)),
    is_capture          INTEGER NOT NULL DEFAULT 0 CHECK (is_capture IN (0,1)),
    eval_centipawns     INTEGER,
    eval_mate_in        INTEGER,
    annotation          TEXT,
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (game_id, ply_number)
);

CREATE INDEX idx_moves_game ON moves(game_id);

CREATE TABLE game_chat_messages (
    chat_message_id  INTEGER PRIMARY KEY,
    game_id          INTEGER NOT NULL REFERENCES games(game_id) ON DELETE CASCADE,
    user_id          INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    message          TEXT NOT NULL,
    sent_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_game_chat_game ON game_chat_messages(game_id);

CREATE TABLE tournaments (
    tournament_id              INTEGER PRIMARY KEY,
    name                       TEXT NOT NULL,
    slug                       TEXT NOT NULL UNIQUE,
    format                     TEXT NOT NULL
                                   CHECK (format IN ('swiss','round_robin','knockout','arena','team_swiss')),
    variant_id                 INTEGER NOT NULL REFERENCES game_variants(variant_id),
    time_control_initial_s     INTEGER NOT NULL,
    time_control_increment_s   INTEGER NOT NULL DEFAULT 0,
    organizer_player_id        INTEGER REFERENCES players(player_id) ON DELETE SET NULL,
    club_id                    INTEGER REFERENCES clubs(club_id) ON DELETE SET NULL,
    min_rating                 INTEGER,
    max_rating                 INTEGER,
    max_participants           INTEGER,
    total_rounds               INTEGER,
    status                     TEXT NOT NULL DEFAULT 'scheduled'
                                   CHECK (status IN ('scheduled','ongoing','completed','cancelled')),
    starts_at                  TEXT NOT NULL,
    ends_at                    TEXT,
    created_at                 TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_deleted                 INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE INDEX idx_tournaments_slug    ON tournaments(slug);
CREATE INDEX idx_tournaments_status  ON tournaments(status);

CREATE TABLE tournament_participants (
    tournament_participant_id  INTEGER PRIMARY KEY,
    tournament_id              INTEGER NOT NULL REFERENCES tournaments(tournament_id) ON DELETE CASCADE,
    player_id                  INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    team_id                    INTEGER REFERENCES teams(team_id) ON DELETE SET NULL,
    seed_rating                INTEGER,
    score                      REAL NOT NULL DEFAULT 0,
    tiebreak_score             REAL NOT NULL DEFAULT 0,
    rank                       INTEGER,
    withdrew_at                TEXT,
    registered_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (tournament_id, player_id)
);

CREATE INDEX idx_tourn_participants_tournament  ON tournament_participants(tournament_id);
CREATE INDEX idx_tourn_participants_player      ON tournament_participants(player_id);

CREATE TABLE tournament_rounds (
    tournament_round_id  INTEGER PRIMARY KEY,
    tournament_id         INTEGER NOT NULL REFERENCES tournaments(tournament_id) ON DELETE CASCADE,
    round_number          INTEGER NOT NULL,
    started_at            TEXT,
    ended_at              TEXT,
    UNIQUE (tournament_id, round_number)
);

CREATE TABLE tournament_pairings (
    tournament_pairing_id  INTEGER PRIMARY KEY,
    tournament_round_id    INTEGER NOT NULL REFERENCES tournament_rounds(tournament_round_id) ON DELETE CASCADE,
    board_number           INTEGER,
    white_participant_id   INTEGER REFERENCES tournament_participants(tournament_participant_id),
    black_participant_id   INTEGER REFERENCES tournament_participants(tournament_participant_id),
    game_id                INTEGER REFERENCES games(game_id) ON DELETE SET NULL,
    result                 TEXT CHECK (result IN ('1-0','0-1','1/2-1/2','bye','*'))
);

CREATE INDEX idx_pairings_round ON tournament_pairings(tournament_round_id);

CREATE TABLE arenas (
    arena_id                   INTEGER PRIMARY KEY,
    name                       TEXT NOT NULL,
    slug                       TEXT NOT NULL UNIQUE,
    variant_id                 INTEGER NOT NULL REFERENCES game_variants(variant_id),
    time_control_initial_s     INTEGER NOT NULL,
    time_control_increment_s   INTEGER NOT NULL DEFAULT 0,
    duration_minutes           INTEGER NOT NULL,
    starts_at                  TEXT NOT NULL,
    ends_at                    TEXT NOT NULL,
    min_rating                 INTEGER,
    max_rating                 INTEGER,
    is_rated                   INTEGER NOT NULL DEFAULT 1 CHECK (is_rated IN (0,1)),
    status                     TEXT NOT NULL DEFAULT 'scheduled'
                                   CHECK (status IN ('scheduled','ongoing','completed','cancelled')),
    created_at                 TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_arenas_slug    ON arenas(slug);
CREATE INDEX idx_arenas_status  ON arenas(status);

CREATE TABLE arena_participants (
    arena_participant_id  INTEGER PRIMARY KEY,
    arena_id               INTEGER NOT NULL REFERENCES arenas(arena_id) ON DELETE CASCADE,
    player_id              INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    score                  INTEGER NOT NULL DEFAULT 0,
    win_streak             INTEGER NOT NULL DEFAULT 0,
    games_played           INTEGER NOT NULL DEFAULT 0,
    rank                   INTEGER,
    joined_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (arena_id, player_id)
);

CREATE INDEX idx_arena_participants_arena ON arena_participants(arena_id, score DESC);

CREATE TABLE puzzles (
    puzzle_id           INTEGER PRIMARY KEY,
    source_game_id      INTEGER REFERENCES games(game_id) ON DELETE SET NULL,
    fen                 TEXT NOT NULL,
    solution_moves_uci  TEXT NOT NULL,
    rating              INTEGER NOT NULL DEFAULT 1500,
    rating_deviation    REAL NOT NULL DEFAULT 350.0,
    themes              TEXT,
    popularity          INTEGER NOT NULL DEFAULT 0,
    play_count          INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_deleted          INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE INDEX idx_puzzles_rating  ON puzzles(rating);
CREATE INDEX idx_puzzles_themes  ON puzzles(themes);

CREATE TABLE puzzle_attempts (
    puzzle_attempt_id  INTEGER PRIMARY KEY,
    puzzle_id          INTEGER NOT NULL REFERENCES puzzles(puzzle_id) ON DELETE CASCADE,
    player_id          INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    is_solved          INTEGER NOT NULL CHECK (is_solved IN (0,1)),
    time_taken_ms      INTEGER,
    rating_before      INTEGER,
    rating_after       INTEGER,
    attempted_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_puzzle_attempts_player  ON puzzle_attempts(player_id, attempted_at);
CREATE INDEX idx_puzzle_attempts_puzzle  ON puzzle_attempts(puzzle_id);

CREATE TABLE lesson_courses (
    course_id                   INTEGER PRIMARY KEY,
    title                       TEXT NOT NULL,
    slug                        TEXT NOT NULL UNIQUE,
    description                 TEXT,
    author_content_creator_id   INTEGER REFERENCES content_creators(content_creator_id) ON DELETE SET NULL,
    difficulty_level            TEXT CHECK (difficulty_level IN ('beginner','intermediate','advanced','expert')),
    is_premium                  INTEGER NOT NULL DEFAULT 0 CHECK (is_premium IN (0,1)),
    published_at                TEXT,
    created_at                  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_deleted                  INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE TABLE lessons (
    lesson_id            INTEGER PRIMARY KEY,
    course_id            INTEGER NOT NULL REFERENCES lesson_courses(course_id) ON DELETE CASCADE,
    title                TEXT NOT NULL,
    sequence_number      INTEGER NOT NULL,
    content_markdown     TEXT,
    starting_fen         TEXT,
    solution_moves_uci   TEXT,
    video_url            TEXT,
    estimated_minutes    INTEGER,
    UNIQUE (course_id, sequence_number)
);

CREATE TABLE lesson_progress (
    lesson_progress_id  INTEGER PRIMARY KEY,
    lesson_id           INTEGER NOT NULL REFERENCES lessons(lesson_id) ON DELETE CASCADE,
    player_id           INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    is_completed         INTEGER NOT NULL DEFAULT 0 CHECK (is_completed IN (0,1)),
    score                REAL,
    completed_at         TEXT,
    UNIQUE (lesson_id, player_id)
);

CREATE INDEX idx_lesson_progress_player ON lesson_progress(player_id);

CREATE TABLE articles (
    article_id                  INTEGER PRIMARY KEY,
    author_content_creator_id   INTEGER REFERENCES content_creators(content_creator_id) ON DELETE SET NULL,
    title                       TEXT NOT NULL,
    slug                        TEXT NOT NULL UNIQUE,
    summary                     TEXT,
    body_markdown               TEXT NOT NULL,
    cover_image_url             TEXT,
    tags                        TEXT,
    linked_game_id              INTEGER REFERENCES games(game_id) ON DELETE SET NULL,
    view_count                  INTEGER NOT NULL DEFAULT 0,
    published_at                TEXT,
    created_at                  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at                  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_deleted                  INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE INDEX idx_articles_slug    ON articles(slug);
CREATE INDEX idx_articles_author  ON articles(author_content_creator_id);

CREATE TABLE article_comments (
    article_comment_id  INTEGER PRIMARY KEY,
    article_id           INTEGER NOT NULL REFERENCES articles(article_id) ON DELETE CASCADE,
    user_id               INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    parent_comment_id     INTEGER REFERENCES article_comments(article_comment_id) ON DELETE CASCADE,
    body                  TEXT NOT NULL,
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    is_deleted            INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0,1))
);

CREATE INDEX idx_article_comments_article ON article_comments(article_id);

CREATE TABLE openings (
    opening_id          INTEGER PRIMARY KEY,
    eco_code            TEXT NOT NULL,
    name                TEXT NOT NULL,
    fen                 TEXT NOT NULL UNIQUE,
    pgn_moves           TEXT NOT NULL,
    parent_opening_id   INTEGER REFERENCES openings(opening_id) ON DELETE SET NULL,
    ply_depth           INTEGER NOT NULL
);

CREATE INDEX idx_openings_eco     ON openings(eco_code);
CREATE INDEX idx_openings_parent  ON openings(parent_opening_id);

CREATE TABLE opening_explorer_stats (
    opening_stat_id       INTEGER PRIMARY KEY,
    opening_id            INTEGER NOT NULL REFERENCES openings(opening_id) ON DELETE CASCADE,
    next_move_san         TEXT NOT NULL,
    next_move_uci         TEXT NOT NULL,
    rating_band           TEXT NOT NULL DEFAULT 'all'
                              CHECK (rating_band IN ('all','0-1200','1200-1600','1600-2000','2000-2400','2400+')),
    variant_id            INTEGER NOT NULL REFERENCES game_variants(variant_id),
    white_wins            INTEGER NOT NULL DEFAULT 0,
    black_wins            INTEGER NOT NULL DEFAULT 0,
    draws                 INTEGER NOT NULL DEFAULT 0,
    total_games           INTEGER NOT NULL DEFAULT 0,
    avg_rating            INTEGER,
    last_recomputed_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (opening_id, next_move_uci, rating_band, variant_id)
);

CREATE INDEX idx_explorer_stats_opening ON opening_explorer_stats(opening_id, rating_band);

CREATE TABLE player_opening_stats (
    player_opening_stat_id  INTEGER PRIMARY KEY,
    player_id               INTEGER NOT NULL REFERENCES players(player_id) ON DELETE CASCADE,
    opening_id               INTEGER NOT NULL REFERENCES openings(opening_id) ON DELETE CASCADE,
    color                     TEXT NOT NULL CHECK (color IN ('white','black')),
    games_played              INTEGER NOT NULL DEFAULT 0,
    wins                      INTEGER NOT NULL DEFAULT 0,
    losses                    INTEGER NOT NULL DEFAULT 0,
    draws                     INTEGER NOT NULL DEFAULT 0,
    updated_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (player_id, opening_id, color)
);

CREATE INDEX idx_player_opening_stats_player ON player_opening_stats(player_id);

CREATE TABLE follows (
    follow_id          INTEGER PRIMARY KEY,
    follower_user_id   INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    followed_user_id   INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    UNIQUE (follower_user_id, followed_user_id),
    CHECK (follower_user_id <> followed_user_id)
);

CREATE INDEX idx_follows_follower  ON follows(follower_user_id);
CREATE INDEX idx_follows_followed  ON follows(followed_user_id);

CREATE TABLE notifications (
    notification_id  INTEGER PRIMARY KEY,
    user_id           INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    type              TEXT NOT NULL,
    payload_json      TEXT,
    is_read           INTEGER NOT NULL DEFAULT 0 CHECK (is_read IN (0,1)),
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX idx_notifications_user ON notifications(user_id, is_read);

INSERT INTO game_variants (code, name, description) VALUES
    ('bullet',        'Bullet',           'Under 3 minutes per side'),
    ('blitz',         'Blitz',            '3-10 minutes per side'),
    ('rapid',         'Rapid',            '10-60 minutes per side'),
    ('classical',     'Classical',        'Over 60 minutes per side'),
    ('chess960',      'Chess960',         'Fischer Random Chess'),
    ('crazyhouse',    'Crazyhouse',       'Captured pieces can be dropped back in'),
    ('antichess',     'Antichess',        'Objective is to lose all pieces'),
    ('atomic',        'Atomic',           'Captures cause explosions'),
    ('horde',         'Horde',            'White has a horde of pawns'),
    ('kingofthehill', 'King of the Hill', 'Win by bringing king to center'),
    ('threecheck',    'Three-check',      'Win by checking opponent three times'),
    ('puzzle',        'Puzzle',           'Tactical puzzle solving mode');
`
