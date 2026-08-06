package transit

const Schema = `-- Ride-hailing service schema

PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id             TEXT PRIMARY KEY,
    first_name          TEXT NOT NULL,
    last_name           TEXT NOT NULL,
    email               TEXT NOT NULL UNIQUE,
    phone_number        TEXT NOT NULL UNIQUE,
    password_hash       TEXT NOT NULL,
    profile_photo_url   TEXT,
    date_of_birth       DATE,
    status              TEXT NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active','suspended','banned','deleted')),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE riders (
    rider_id            TEXT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    default_payment_method_id TEXT,
    home_address        TEXT,
    work_address         TEXT,
    rider_rating_avg     REAL DEFAULT 5.0 CHECK (rider_rating_avg BETWEEN 0 AND 5),
    total_trips          INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE drivers (
    driver_id             TEXT PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    license_number        TEXT NOT NULL UNIQUE,
    license_expiry        DATE NOT NULL,
    driver_rating_avg     REAL DEFAULT 5.0 CHECK (driver_rating_avg BETWEEN 0 AND 5),
    total_trips           INTEGER NOT NULL DEFAULT 0,
    background_check_status TEXT NOT NULL DEFAULT 'pending'
                           CHECK (background_check_status IN ('pending','approved','rejected')),
    onboarding_status      TEXT NOT NULL DEFAULT 'pending'
                           CHECK (onboarding_status IN ('pending','active','inactive','deactivated')),
    payout_account_id     TEXT,
    created_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE vehicle_types (
    vehicle_type_id       TEXT PRIMARY KEY,
    name                  TEXT NOT NULL,
    description           TEXT,
    capacity              INTEGER NOT NULL,
    base_fare_multiplier  REAL NOT NULL DEFAULT 1.0
);

CREATE TABLE vehicles (
    vehicle_id            TEXT PRIMARY KEY,
    driver_id             TEXT NOT NULL REFERENCES drivers(driver_id) ON DELETE CASCADE,
    vehicle_type_id       TEXT NOT NULL REFERENCES vehicle_types(vehicle_type_id),
    make                  TEXT NOT NULL,
    model                 TEXT NOT NULL,
    year                  INTEGER NOT NULL,
    color                 TEXT,
    license_plate         TEXT NOT NULL UNIQUE,
    vin                   TEXT UNIQUE,
    seats                 INTEGER NOT NULL,
    inspection_status     TEXT NOT NULL DEFAULT 'pending'
                          CHECK (inspection_status IN ('pending','passed','failed')),
    is_active             INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_vehicles_driver ON vehicles(driver_id);

CREATE TABLE locations (
    location_id           TEXT PRIMARY KEY,
    label                 TEXT,
    address_line          TEXT NOT NULL,
    city                  TEXT NOT NULL,
    state_province        TEXT,
    postal_code           TEXT,
    country               TEXT NOT NULL,
    latitude              REAL NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude             REAL NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    created_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_locations_lat_lng ON locations(latitude, longitude);

CREATE TABLE driver_location_pings (
    ping_id                INTEGER PRIMARY KEY AUTOINCREMENT,
    driver_id              TEXT NOT NULL REFERENCES drivers(driver_id) ON DELETE CASCADE,
    trip_id                TEXT REFERENCES trips(trip_id) ON DELETE SET NULL,
    latitude               REAL NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude              REAL NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    heading_degrees        REAL,
    speed_mps              REAL,
    recorded_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_pings_driver_time ON driver_location_pings(driver_id, recorded_at);
CREATE INDEX idx_pings_trip ON driver_location_pings(trip_id);

CREATE TABLE service_zones (
    zone_id                TEXT PRIMARY KEY,
    name                   TEXT NOT NULL,
    city                   TEXT NOT NULL,
    country                TEXT NOT NULL,
    min_latitude           REAL NOT NULL,
    max_latitude           REAL NOT NULL,
    min_longitude          REAL NOT NULL,
    max_longitude          REAL NOT NULL,
    is_active              INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1))
);

CREATE TABLE pricing_rules (
    pricing_rule_id        TEXT PRIMARY KEY,
    zone_id                TEXT NOT NULL REFERENCES service_zones(zone_id),
    vehicle_type_id        TEXT NOT NULL REFERENCES vehicle_types(vehicle_type_id),
    base_fare              REAL NOT NULL DEFAULT 2.50,
    cost_per_km            REAL NOT NULL DEFAULT 1.20,
    cost_per_minute        REAL NOT NULL DEFAULT 0.25,
    minimum_fare           REAL NOT NULL DEFAULT 5.00,
    booking_fee            REAL NOT NULL DEFAULT 1.00,
    cancellation_fee       REAL NOT NULL DEFAULT 5.00,
    currency               TEXT NOT NULL DEFAULT 'USD',
    effective_from         TEXT NOT NULL DEFAULT (datetime('now')),
    effective_to           TEXT
);

CREATE INDEX idx_pricing_zone_vehicle ON pricing_rules(zone_id, vehicle_type_id);

CREATE TABLE surge_pricing (
    surge_id               TEXT PRIMARY KEY,
    zone_id                TEXT NOT NULL REFERENCES service_zones(zone_id),
    multiplier             REAL NOT NULL CHECK (multiplier >= 1.0),
    reason                 TEXT,
    starts_at              TEXT NOT NULL,
    ends_at                TEXT
);

CREATE INDEX idx_surge_zone_time ON surge_pricing(zone_id, starts_at, ends_at);

CREATE TABLE promotions (
    promo_id               TEXT PRIMARY KEY,
    code                   TEXT NOT NULL UNIQUE,
    description             TEXT,
    discount_type          TEXT NOT NULL CHECK (discount_type IN ('percentage','flat')),
    discount_value         REAL NOT NULL,
    max_discount_amount    REAL,
    valid_from             TEXT NOT NULL,
    valid_to               TEXT NOT NULL,
    usage_limit_per_user   INTEGER NOT NULL DEFAULT 1,
    is_active              INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1))
);

CREATE TABLE promotion_redemptions (
    redemption_id          TEXT PRIMARY KEY,
    promo_id               TEXT NOT NULL REFERENCES promotions(promo_id),
    rider_id               TEXT NOT NULL REFERENCES riders(rider_id),
    trip_id                TEXT REFERENCES trips(trip_id),
    redeemed_at            TEXT NOT NULL DEFAULT (datetime('now')),
    discount_applied       REAL NOT NULL
);

CREATE TABLE trips (
    trip_id                TEXT PRIMARY KEY,
    rider_id               TEXT NOT NULL REFERENCES riders(rider_id),
    driver_id              TEXT REFERENCES drivers(driver_id),
    vehicle_id             TEXT REFERENCES vehicles(vehicle_id),
    vehicle_type_id        TEXT NOT NULL REFERENCES vehicle_types(vehicle_type_id),
    zone_id                TEXT REFERENCES service_zones(zone_id),

    pickup_location_id     TEXT NOT NULL REFERENCES locations(location_id),
    dropoff_location_id    TEXT NOT NULL REFERENCES locations(location_id),

    status                 TEXT NOT NULL DEFAULT 'requested'
                            CHECK (status IN (
                                'requested','matched','driver_arriving','in_progress',
                                'completed','cancelled_by_rider','cancelled_by_driver','no_show'
                            )),

    requested_at            TEXT NOT NULL DEFAULT (datetime('now')),
    matched_at               TEXT,
    driver_arrived_at        TEXT,
    trip_started_at          TEXT,
    trip_completed_at        TEXT,
    cancelled_at              TEXT,
    cancellation_reason       TEXT,

    estimated_distance_km     REAL,
    actual_distance_km        REAL,
    estimated_duration_min    REAL,
    actual_duration_min       REAL,

    created_at                TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at                TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_trips_rider ON trips(rider_id);
CREATE INDEX idx_trips_driver ON trips(driver_id);
CREATE INDEX idx_trips_status ON trips(status);
CREATE INDEX idx_trips_requested_at ON trips(requested_at);

CREATE TABLE trip_fares (
    trip_id                TEXT PRIMARY KEY REFERENCES trips(trip_id) ON DELETE CASCADE,
    pricing_rule_id        TEXT REFERENCES pricing_rules(pricing_rule_id),
    surge_id               TEXT REFERENCES surge_pricing(surge_id),
    base_fare              REAL NOT NULL,
    distance_fare          REAL NOT NULL,
    time_fare              REAL NOT NULL,
    surge_multiplier       REAL NOT NULL DEFAULT 1.0,
    booking_fee            REAL NOT NULL DEFAULT 0,
    promo_discount         REAL NOT NULL DEFAULT 0,
    tip_amount             REAL NOT NULL DEFAULT 0,
    tax_amount             REAL NOT NULL DEFAULT 0,
    total_fare             REAL NOT NULL,
    currency               TEXT NOT NULL DEFAULT 'USD',
    calculated_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE payment_methods (
    payment_method_id      TEXT PRIMARY KEY,
    user_id                TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    method_type            TEXT NOT NULL CHECK (method_type IN ('credit_card','debit_card','paypal','wallet','cash')),
    provider               TEXT,
    last_four              TEXT,
    expiry_month           INTEGER,
    expiry_year            INTEGER,
    is_default             INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)),
    created_at              TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_payment_methods_user ON payment_methods(user_id);

CREATE TABLE payments (
    payment_id              TEXT PRIMARY KEY,
    trip_id                 TEXT NOT NULL REFERENCES trips(trip_id),
    payment_method_id       TEXT REFERENCES payment_methods(payment_method_id),
    amount                  REAL NOT NULL,
    currency                TEXT NOT NULL DEFAULT 'USD',
    status                  TEXT NOT NULL DEFAULT 'pending'
                             CHECK (status IN ('pending','authorized','captured','failed','refunded')),
    transaction_ref          TEXT,
    processed_at              TEXT,
    created_at                TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_payments_trip ON payments(trip_id);

CREATE TABLE driver_payouts (
    payout_id               TEXT PRIMARY KEY,
    driver_id               TEXT NOT NULL REFERENCES drivers(driver_id),
    period_start             TEXT NOT NULL,
    period_end               TEXT NOT NULL,
    gross_earnings           REAL NOT NULL,
    platform_fee             REAL NOT NULL,
    net_payout               REAL NOT NULL,
    status                   TEXT NOT NULL DEFAULT 'pending'
                             CHECK (status IN ('pending','processing','paid','failed')),
    paid_at                   TEXT,
    created_at                TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_payouts_driver ON driver_payouts(driver_id);

CREATE TABLE ratings (
    rating_id               TEXT PRIMARY KEY,
    trip_id                 TEXT NOT NULL REFERENCES trips(trip_id) ON DELETE CASCADE,
    rater_user_id            TEXT NOT NULL REFERENCES users(user_id),
    ratee_user_id             TEXT NOT NULL REFERENCES users(user_id),
    rating_direction          TEXT NOT NULL CHECK (rating_direction IN ('rider_to_driver','driver_to_rider')),
    score                     INTEGER NOT NULL CHECK (score BETWEEN 1 AND 5),
    comment                   TEXT,
    created_at                 TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (trip_id, rating_direction)
);

CREATE INDEX idx_ratings_ratee ON ratings(ratee_user_id);
CREATE INDEX idx_ratings_trip ON ratings(trip_id);

CREATE TABLE rating_tags (
    tag_id                   TEXT PRIMARY KEY,
    label                    TEXT NOT NULL,
    sentiment                TEXT NOT NULL CHECK (sentiment IN ('positive','negative'))
);

CREATE TABLE rating_tag_links (
    rating_id                TEXT NOT NULL REFERENCES ratings(rating_id) ON DELETE CASCADE,
    tag_id                    TEXT NOT NULL REFERENCES rating_tags(tag_id),
    PRIMARY KEY (rating_id, tag_id)
);

CREATE TABLE trip_events (
    event_id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    trip_id                    TEXT NOT NULL REFERENCES trips(trip_id) ON DELETE CASCADE,
    event_type                 TEXT NOT NULL,
    event_payload               TEXT,
    created_at                   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_trip_events_trip ON trip_events(trip_id, created_at);

CREATE VIEW vw_completed_trip_summary AS
SELECT
    t.trip_id,
    t.rider_id,
    t.driver_id,
    t.vehicle_id,
    t.trip_started_at,
    t.trip_completed_at,
    t.actual_distance_km,
    t.actual_duration_min,
    f.total_fare,
    f.currency,
    r1.score AS rider_rating_given,
    r2.score AS driver_rating_given
FROM trips t
JOIN trip_fares f ON f.trip_id = t.trip_id
LEFT JOIN ratings r1 ON r1.trip_id = t.trip_id AND r1.rating_direction = 'rider_to_driver'
LEFT JOIN ratings r2 ON r2.trip_id = t.trip_id AND r2.rating_direction = 'driver_to_rider'
WHERE t.status = 'completed';

CREATE VIEW vw_driver_last_known_location AS
SELECT
    d.driver_id,
    p.latitude,
    p.longitude,
    p.recorded_at
FROM drivers d
JOIN driver_location_pings p ON p.driver_id = d.driver_id
WHERE p.recorded_at = (
    SELECT MAX(p2.recorded_at)
    FROM driver_location_pings p2
    WHERE p2.driver_id = d.driver_id
);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
FOR EACH ROW
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE user_id = OLD.user_id;
END;

CREATE TRIGGER trg_trips_updated_at
AFTER UPDATE ON trips
FOR EACH ROW
BEGIN
    UPDATE trips SET updated_at = datetime('now') WHERE trip_id = OLD.trip_id;
END;

CREATE TRIGGER trg_trip_completed_counters
AFTER UPDATE OF status ON trips
FOR EACH ROW
WHEN NEW.status = 'completed' AND OLD.status != 'completed'
BEGIN
    UPDATE riders SET total_trips = total_trips + 1 WHERE rider_id = NEW.rider_id;
    UPDATE drivers SET total_trips = total_trips + 1 WHERE driver_id = NEW.driver_id;
END;

INSERT INTO vehicle_types (vehicle_type_id, name, description, capacity, base_fare_multiplier) VALUES
    ('economy', 'Economy', 'Affordable everyday rides', 4, 1.0),
    ('xl',      'XL',      'Larger vehicles for groups', 6, 1.5),
    ('premium', 'Premium', 'Luxury vehicles', 4, 2.0),
    ('pool',    'Pool',    'Shared rides with other riders', 4, 0.7);

INSERT INTO rating_tags (tag_id, label, sentiment) VALUES
    ('clean_vehicle', 'Clean Vehicle', 'positive'),
    ('great_conversation', 'Great Conversation', 'positive'),
    ('safe_driving', 'Safe Driving', 'positive'),
    ('rude_behavior', 'Rude Behavior', 'negative'),
    ('late_arrival', 'Late Arrival', 'negative'),
    ('unsafe_driving', 'Unsafe Driving', 'negative');
`
