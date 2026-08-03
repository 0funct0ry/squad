package stay

const Schema = `
-- Minimal peer-to-peer lodging marketplace schema

PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    email               TEXT NOT NULL UNIQUE,
    phone_number        TEXT UNIQUE,
    password_hash       TEXT NOT NULL,
    first_name          TEXT NOT NULL,
    last_name           TEXT NOT NULL,
    date_of_birth       DATE,
    profile_photo_url   TEXT,
    bio                 TEXT,
    government_id_verified INTEGER NOT NULL DEFAULT 0 CHECK (government_id_verified IN (0,1)),
    email_verified      INTEGER NOT NULL DEFAULT 0 CHECK (email_verified IN (0,1)),
    phone_verified      INTEGER NOT NULL DEFAULT 0 CHECK (phone_verified IN (0,1)),
    preferred_locale    TEXT DEFAULT 'en-US',
    preferred_currency  TEXT DEFAULT 'USD',
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    is_active           INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1))
);

CREATE INDEX idx_users_email ON users(email);

CREATE TABLE hosts (
    host_id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                 INTEGER NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
    host_since              DATE NOT NULL DEFAULT (date('now')),
    is_superhost            INTEGER NOT NULL DEFAULT 0 CHECK (is_superhost IN (0,1)),
    response_rate_pct       REAL CHECK (response_rate_pct BETWEEN 0 AND 100),
    response_time_minutes   INTEGER,
    identity_verified       INTEGER NOT NULL DEFAULT 0 CHECK (identity_verified IN (0,1)),
    payout_method           TEXT CHECK (payout_method IN ('bank_transfer','paypal','debit_card')),
    payout_account_ref      TEXT,
    created_at              TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE guests (
    guest_id                INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                 INTEGER NOT NULL UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
    preferred_payment_method TEXT CHECK (preferred_payment_method IN ('credit_card','debit_card','paypal','apple_pay','google_pay')),
    default_payment_ref     TEXT,
    created_at              TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE properties (
    property_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    host_id              INTEGER NOT NULL REFERENCES hosts(host_id) ON DELETE CASCADE,
    property_type        TEXT NOT NULL CHECK (property_type IN
                          ('apartment','house','condo','townhouse','villa','cabin','cottage',
                           'bungalow','loft','guesthouse','tent','boat','other')),
    room_type            TEXT NOT NULL CHECK (room_type IN ('entire_place','private_room','shared_room','hotel_room')),
    address_line1        TEXT NOT NULL,
    address_line2        TEXT,
    city                  TEXT NOT NULL,
    state_province        TEXT,
    postal_code           TEXT,
    country_code          TEXT NOT NULL,
    latitude              REAL NOT NULL,
    longitude             REAL NOT NULL,
    max_guests            INTEGER NOT NULL CHECK (max_guests > 0),
    bedrooms              INTEGER NOT NULL DEFAULT 0 CHECK (bedrooms >= 0),
    beds                  INTEGER NOT NULL DEFAULT 0 CHECK (beds >= 0),
    bathrooms             REAL NOT NULL DEFAULT 0 CHECK (bathrooms >= 0),
    square_feet           REAL,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_properties_host ON properties(host_id);
CREATE INDEX idx_properties_geo ON properties(latitude, longitude);
CREATE INDEX idx_properties_city ON properties(city, country_code);

CREATE TABLE listings (
    listing_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    property_id          INTEGER NOT NULL REFERENCES properties(property_id) ON DELETE CASCADE,
    title                 TEXT NOT NULL,
    description           TEXT,
    house_rules           TEXT,
    cancellation_policy   TEXT NOT NULL DEFAULT 'moderate'
                          CHECK (cancellation_policy IN ('flexible','moderate','strict','super_strict')),
    check_in_time         TEXT NOT NULL DEFAULT '15:00',
    check_out_time        TEXT NOT NULL DEFAULT '11:00',
    min_nights            INTEGER NOT NULL DEFAULT 1 CHECK (min_nights > 0),
    max_nights            INTEGER NOT NULL DEFAULT 365 CHECK (max_nights >= min_nights),
    instant_book_enabled  INTEGER NOT NULL DEFAULT 0 CHECK (instant_book_enabled IN (0,1)),
    status                TEXT NOT NULL DEFAULT 'draft'
                          CHECK (status IN ('draft','active','paused','suspended','deleted')),
    base_currency         TEXT NOT NULL DEFAULT 'USD',
    published_at          TEXT,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_listings_property ON listings(property_id);
CREATE INDEX idx_listings_status ON listings(status);

CREATE TABLE amenities (
    amenity_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    category        TEXT NOT NULL CHECK (category IN
                    ('essentials','features','location','safety','accessibility','outdoor','entertainment')),
    icon_url        TEXT
);

CREATE TABLE listing_amenities (
    listing_id      INTEGER NOT NULL REFERENCES listings(listing_id) ON DELETE CASCADE,
    amenity_id      INTEGER NOT NULL REFERENCES amenities(amenity_id) ON DELETE CASCADE,
    PRIMARY KEY (listing_id, amenity_id)
);

CREATE TABLE listing_photos (
    photo_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_id      INTEGER NOT NULL REFERENCES listings(listing_id) ON DELETE CASCADE,
    photo_url       TEXT NOT NULL,
    caption         TEXT,
    display_order   INTEGER NOT NULL DEFAULT 0,
    is_cover_photo  INTEGER NOT NULL DEFAULT 0 CHECK (is_cover_photo IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_listing_photos_listing ON listing_photos(listing_id);

CREATE TABLE listing_calendar (
    listing_id      INTEGER NOT NULL REFERENCES listings(listing_id) ON DELETE CASCADE,
    calendar_date   DATE NOT NULL,
    is_available    INTEGER NOT NULL DEFAULT 1 CHECK (is_available IN (0,1)),
    blocked_reason  TEXT CHECK (blocked_reason IN ('host_block','booked','maintenance','external_sync', NULL)),
    nightly_price   NUMERIC NOT NULL CHECK (nightly_price >= 0),
    currency        TEXT NOT NULL DEFAULT 'USD',
    min_nights_override INTEGER,
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (listing_id, calendar_date)
);

CREATE INDEX idx_calendar_date ON listing_calendar(calendar_date);
CREATE INDEX idx_calendar_availability ON listing_calendar(listing_id, is_available);

CREATE TABLE listing_base_pricing (
    listing_id          INTEGER PRIMARY KEY REFERENCES listings(listing_id) ON DELETE CASCADE,
    base_nightly_price   NUMERIC NOT NULL CHECK (base_nightly_price >= 0),
    cleaning_fee         NUMERIC NOT NULL DEFAULT 0 CHECK (cleaning_fee >= 0),
    security_deposit      NUMERIC NOT NULL DEFAULT 0 CHECK (security_deposit >= 0),
    extra_guest_fee       NUMERIC NOT NULL DEFAULT 0 CHECK (extra_guest_fee >= 0),
    extra_guest_threshold INTEGER NOT NULL DEFAULT 1,
    weekly_discount_pct   REAL NOT NULL DEFAULT 0 CHECK (weekly_discount_pct BETWEEN 0 AND 100),
    monthly_discount_pct  REAL NOT NULL DEFAULT 0 CHECK (monthly_discount_pct BETWEEN 0 AND 100),
    updated_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE pricing_rules (
    pricing_rule_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_id           INTEGER NOT NULL REFERENCES listings(listing_id) ON DELETE CASCADE,
    rule_name            TEXT NOT NULL,
    rule_type             TEXT NOT NULL CHECK (rule_type IN
                          ('seasonal','weekend','length_of_stay','early_bird','last_minute','custom_date_range','demand_based')),
    adjustment_type       TEXT NOT NULL CHECK (adjustment_type IN ('percentage','fixed_amount')),
    adjustment_value      NUMERIC NOT NULL,
    start_date            DATE,
    end_date              DATE,
    days_of_week_mask     INTEGER,
    min_nights_trigger    INTEGER,
    priority              INTEGER NOT NULL DEFAULT 0,
    is_active             INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (end_date IS NULL OR start_date IS NULL OR end_date >= start_date)
);

CREATE INDEX idx_pricing_rules_listing ON pricing_rules(listing_id);
CREATE INDEX idx_pricing_rules_dates ON pricing_rules(start_date, end_date);

CREATE TABLE bookings (
    booking_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_id            INTEGER NOT NULL REFERENCES listings(listing_id),
    guest_id              INTEGER NOT NULL REFERENCES guests(guest_id),
    check_in_date         DATE NOT NULL,
    check_out_date        DATE NOT NULL,
    num_adults            INTEGER NOT NULL DEFAULT 1 CHECK (num_adults >= 0),
    num_children          INTEGER NOT NULL DEFAULT 0 CHECK (num_children >= 0),
    num_infants           INTEGER NOT NULL DEFAULT 0 CHECK (num_infants >= 0),
    num_pets              INTEGER NOT NULL DEFAULT 0 CHECK (num_pets >= 0),
    nightly_rate_avg      NUMERIC NOT NULL CHECK (nightly_rate_avg >= 0),
    nights_count          INTEGER NOT NULL CHECK (nights_count > 0),
    subtotal              NUMERIC NOT NULL CHECK (subtotal >= 0),
    cleaning_fee           NUMERIC NOT NULL DEFAULT 0,
    service_fee            NUMERIC NOT NULL DEFAULT 0,
    taxes                  NUMERIC NOT NULL DEFAULT 0,
    discount_total         NUMERIC NOT NULL DEFAULT 0,
    total_price            NUMERIC NOT NULL CHECK (total_price >= 0),
    currency               TEXT NOT NULL DEFAULT 'USD',
    status                 TEXT NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending','confirmed','cancelled_by_guest',
                                             'cancelled_by_host','completed','declined','expired')),
    booking_source         TEXT NOT NULL DEFAULT 'website' CHECK (booking_source IN ('website','mobile_app','api')),
    special_requests       TEXT,
    cancelled_at           TEXT,
    cancellation_reason    TEXT,
    created_at             TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at             TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (check_out_date > check_in_date)
);

CREATE INDEX idx_bookings_listing ON bookings(listing_id);
CREATE INDEX idx_bookings_guest ON bookings(guest_id);
CREATE INDEX idx_bookings_dates ON bookings(check_in_date, check_out_date);
CREATE INDEX idx_bookings_status ON bookings(status);

CREATE TABLE booking_guests (
    booking_guest_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    booking_id          INTEGER NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    full_name           TEXT NOT NULL,
    age_group            TEXT NOT NULL DEFAULT 'adult' CHECK (age_group IN ('adult','child','infant'))
);

CREATE INDEX idx_booking_guests_booking ON booking_guests(booking_id);

CREATE TABLE payments (
    payment_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    booking_id           INTEGER NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    payment_type          TEXT NOT NULL CHECK (payment_type IN ('charge','refund','payout')),
    amount                NUMERIC NOT NULL,
    currency              TEXT NOT NULL DEFAULT 'USD',
    payment_method         TEXT CHECK (payment_method IN ('credit_card','debit_card','paypal','apple_pay','google_pay','bank_transfer')),
    processor_reference    TEXT,
    status                 TEXT NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending','succeeded','failed','refunded')),
    processed_at           TEXT,
    created_at             TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_payments_booking ON payments(booking_id);

CREATE TABLE reviews (
    review_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    booking_id           INTEGER NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    reviewer_user_id      INTEGER NOT NULL REFERENCES users(user_id),
    reviewee_user_id      INTEGER NOT NULL REFERENCES users(user_id),
    review_type           TEXT NOT NULL CHECK (review_type IN ('guest_to_host','host_to_guest')),
    overall_rating        INTEGER NOT NULL CHECK (overall_rating BETWEEN 1 AND 5),
    cleanliness_rating    INTEGER CHECK (cleanliness_rating BETWEEN 1 AND 5),
    communication_rating  INTEGER CHECK (communication_rating BETWEEN 1 AND 5),
    checkin_rating        INTEGER CHECK (checkin_rating BETWEEN 1 AND 5),
    accuracy_rating       INTEGER CHECK (accuracy_rating BETWEEN 1 AND 5),
    location_rating       INTEGER CHECK (location_rating BETWEEN 1 AND 5),
    value_rating          INTEGER CHECK (value_rating BETWEEN 1 AND 5),
    comment                TEXT,
    host_response          TEXT,
    host_response_at        TEXT,
    is_public               INTEGER NOT NULL DEFAULT 1 CHECK (is_public IN (0,1)),
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (booking_id, review_type)
);

CREATE INDEX idx_reviews_booking ON reviews(booking_id);
CREATE INDEX idx_reviews_reviewee ON reviews(reviewee_user_id);

CREATE TABLE conversations (
    conversation_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_id            INTEGER REFERENCES listings(listing_id),
    booking_id            INTEGER REFERENCES bookings(booking_id),
    host_user_id          INTEGER NOT NULL REFERENCES users(user_id),
    guest_user_id         INTEGER NOT NULL REFERENCES users(user_id),
    created_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_conversations_host ON conversations(host_user_id);
CREATE INDEX idx_conversations_guest ON conversations(guest_user_id);

CREATE TABLE messages (
    message_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id       INTEGER NOT NULL REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    sender_user_id        INTEGER NOT NULL REFERENCES users(user_id),
    body                   TEXT NOT NULL,
    sent_at                TEXT NOT NULL DEFAULT (datetime('now')),
    read_at                TEXT
);

CREATE INDEX idx_messages_conversation ON messages(conversation_id);

CREATE TABLE wishlists (
    wishlist_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id               INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    name                   TEXT NOT NULL DEFAULT 'My Wishlist',
    is_private             INTEGER NOT NULL DEFAULT 1 CHECK (is_private IN (0,1)),
    created_at             TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE wishlist_items (
    wishlist_id           INTEGER NOT NULL REFERENCES wishlists(wishlist_id) ON DELETE CASCADE,
    listing_id             INTEGER NOT NULL REFERENCES listings(listing_id) ON DELETE CASCADE,
    added_at                TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (wishlist_id, listing_id)
);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE user_id = NEW.user_id;
END;

CREATE TRIGGER trg_listings_updated_at
AFTER UPDATE ON listings
BEGIN
    UPDATE listings SET updated_at = datetime('now') WHERE listing_id = NEW.listing_id;
END;

CREATE TRIGGER trg_properties_updated_at
AFTER UPDATE ON properties
BEGIN
    UPDATE properties SET updated_at = datetime('now') WHERE property_id = NEW.property_id;
END;

CREATE TRIGGER trg_bookings_updated_at
AFTER UPDATE ON bookings
BEGIN
    UPDATE bookings SET updated_at = datetime('now') WHERE booking_id = NEW.booking_id;
END;

INSERT INTO amenities (name, category) VALUES
    ('Wifi', 'essentials'),
    ('Kitchen', 'essentials'),
    ('Washer', 'essentials'),
    ('Air conditioning', 'features'),
    ('Heating', 'features'),
    ('Dedicated workspace', 'features'),
    ('TV', 'entertainment'),
    ('Pool', 'outdoor'),
    ('Free parking on premises', 'location'),
    ('Smoke alarm', 'safety'),
    ('Carbon monoxide alarm', 'safety'),
    ('First aid kit', 'safety'),
    ('Wheelchair accessible', 'accessibility'),
    ('Elevator', 'accessibility'),
    ('Backyard', 'outdoor');
`
