package food_delivery

const Scheme = `-- Food Delivery Platform Schema

PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name       TEXT NOT NULL,
    email           TEXT NOT NULL UNIQUE,
    phone           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    user_type       TEXT NOT NULL CHECK (user_type IN ('customer','restaurant_owner','driver','admin')),
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE addresses (
    address_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL,
    label           TEXT DEFAULT 'Home',
    line1           TEXT NOT NULL,
    line2           TEXT,
    city            TEXT NOT NULL,
    state           TEXT,
    postal_code     TEXT,
    country         TEXT NOT NULL DEFAULT 'US',
    latitude        REAL,
    longitude       REAL,
    is_default      INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

-- ------------------------------------------------------------
-- RESTAURANTS
-- ------------------------------------------------------------

CREATE TABLE restaurants (
    restaurant_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id        INTEGER NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    cuisine_type    TEXT,
    phone           TEXT,
    email           TEXT,
    address_line1   TEXT NOT NULL,
    address_line2   TEXT,
    city            TEXT NOT NULL,
    state           TEXT,
    postal_code     TEXT,
    country         TEXT NOT NULL DEFAULT 'US',
    latitude        REAL,
    longitude       REAL,
    price_range     INTEGER CHECK (price_range BETWEEN 1 AND 4),
    avg_prep_time_minutes INTEGER DEFAULT 20,
    is_open         INTEGER NOT NULL DEFAULT 1 CHECK (is_open IN (0,1)),
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (owner_id) REFERENCES users(user_id) ON DELETE RESTRICT
);

CREATE TABLE restaurant_hours (
    hours_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    restaurant_id   INTEGER NOT NULL,
    day_of_week     INTEGER NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    open_time       TEXT NOT NULL,
    close_time      TEXT NOT NULL,
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(restaurant_id) ON DELETE CASCADE
);

CREATE TABLE menu_categories (
    category_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    restaurant_id   INTEGER NOT NULL,
    name            TEXT NOT NULL,
    display_order   INTEGER DEFAULT 0,
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(restaurant_id) ON DELETE CASCADE
);

CREATE TABLE menu_items (
    item_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    restaurant_id   INTEGER NOT NULL,
    category_id     INTEGER,
    name            TEXT NOT NULL,
    description     TEXT,
    price           NUMERIC NOT NULL CHECK (price >= 0),
    is_vegetarian   INTEGER NOT NULL DEFAULT 0 CHECK (is_vegetarian IN (0,1)),
    is_vegan        INTEGER NOT NULL DEFAULT 0 CHECK (is_vegan IN (0,1)),
    is_available    INTEGER NOT NULL DEFAULT 1 CHECK (is_available IN (0,1)),
    calories        INTEGER,
    image_url       TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(restaurant_id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES menu_categories(category_id) ON DELETE SET NULL
);

CREATE TABLE modifier_groups (
    modifier_group_id  INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id             INTEGER NOT NULL,
    name                TEXT NOT NULL,
    is_required         INTEGER NOT NULL DEFAULT 0 CHECK (is_required IN (0,1)),
    min_selections      INTEGER NOT NULL DEFAULT 0,
    max_selections      INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY (item_id) REFERENCES menu_items(item_id) ON DELETE CASCADE
);

CREATE TABLE modifier_options (
    modifier_option_id  INTEGER PRIMARY KEY AUTOINCREMENT,
    modifier_group_id   INTEGER NOT NULL,
    name                 TEXT NOT NULL,
    extra_price          NUMERIC NOT NULL DEFAULT 0,
    is_available         INTEGER NOT NULL DEFAULT 1 CHECK (is_available IN (0,1)),
    FOREIGN KEY (modifier_group_id) REFERENCES modifier_groups(modifier_group_id) ON DELETE CASCADE
);

CREATE TABLE drivers (
    driver_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL UNIQUE,
    vehicle_type    TEXT CHECK (vehicle_type IN ('bike','scooter','car','on_foot')),
    license_plate   TEXT,
    is_available    INTEGER NOT NULL DEFAULT 0 CHECK (is_available IN (0,1)),
    current_latitude   REAL,
    current_longitude  REAL,
    rating_avg      NUMERIC DEFAULT 0,
    total_deliveries INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE orders (
    order_id            INTEGER PRIMARY KEY AUTOINCREMENT,
    customer_id         INTEGER NOT NULL,
    restaurant_id       INTEGER NOT NULL,
    driver_id           INTEGER,
    delivery_address_id INTEGER NOT NULL,
    status              TEXT NOT NULL DEFAULT 'placed'
                         CHECK (status IN ('placed','confirmed','preparing','ready_for_pickup',
                                            'out_for_delivery','delivered','cancelled')),
    subtotal            NUMERIC NOT NULL DEFAULT 0,
    delivery_fee        NUMERIC NOT NULL DEFAULT 0,
    tax_amount          NUMERIC NOT NULL DEFAULT 0,
    tip_amount          NUMERIC NOT NULL DEFAULT 0,
    discount_amount     NUMERIC NOT NULL DEFAULT 0,
    total_amount        NUMERIC NOT NULL DEFAULT 0,
    payment_id          INTEGER,
    placed_at           TEXT NOT NULL DEFAULT (datetime('now')),
    confirmed_at        TEXT,
    ready_at             TEXT,
    picked_up_at         TEXT,
    delivered_at          TEXT,
    cancelled_at        TEXT,
    cancellation_reason  TEXT,
    special_instructions TEXT,
    FOREIGN KEY (customer_id) REFERENCES users(user_id) ON DELETE RESTRICT,
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(restaurant_id) ON DELETE RESTRICT,
    FOREIGN KEY (driver_id) REFERENCES drivers(driver_id) ON DELETE SET NULL,
    FOREIGN KEY (delivery_address_id) REFERENCES addresses(address_id) ON DELETE RESTRICT
);

CREATE TABLE order_items (
    order_item_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id        INTEGER NOT NULL,
    item_id         INTEGER NOT NULL,
    quantity        INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price      NUMERIC NOT NULL,
    line_total      NUMERIC NOT NULL,
    notes           TEXT,
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES menu_items(item_id) ON DELETE RESTRICT
);

CREATE TABLE order_item_modifiers (
    order_item_modifier_id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_item_id           INTEGER NOT NULL,
    modifier_option_id      INTEGER NOT NULL,
    extra_price              NUMERIC NOT NULL DEFAULT 0,
    FOREIGN KEY (order_item_id) REFERENCES order_items(order_item_id) ON DELETE CASCADE,
    FOREIGN KEY (modifier_option_id) REFERENCES modifier_options(modifier_option_id) ON DELETE RESTRICT
);

CREATE TABLE order_status_history (
    history_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id        INTEGER NOT NULL,
    status          TEXT NOT NULL,
    changed_at      TEXT NOT NULL DEFAULT (datetime('now')),
    note            TEXT,
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE
);

CREATE TABLE payments (
    payment_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id        INTEGER NOT NULL UNIQUE,
    user_id         INTEGER NOT NULL,
    amount          NUMERIC NOT NULL,
    payment_method  TEXT NOT NULL CHECK (payment_method IN ('credit_card','debit_card','paypal','wallet','cash')),
    payment_status  TEXT NOT NULL DEFAULT 'pending'
                     CHECK (payment_status IN ('pending','completed','failed','refunded')),
    transaction_ref TEXT,
    paid_at         TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE RESTRICT
);

CREATE TABLE restaurant_ratings (
    rating_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id        INTEGER NOT NULL UNIQUE,
    customer_id     INTEGER NOT NULL,
    restaurant_id   INTEGER NOT NULL,
    rating          INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    review_text     TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (customer_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(restaurant_id) ON DELETE CASCADE
);

CREATE TABLE driver_ratings (
    rating_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id        INTEGER NOT NULL UNIQUE,
    customer_id     INTEGER NOT NULL,
    driver_id       INTEGER NOT NULL,
    rating          INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    review_text     TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (customer_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (driver_id) REFERENCES drivers(driver_id) ON DELETE CASCADE
);

CREATE TABLE menu_item_ratings (
    rating_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    order_item_id   INTEGER NOT NULL,
    customer_id     INTEGER NOT NULL,
    item_id         INTEGER NOT NULL,
    rating          INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (order_item_id) REFERENCES order_items(order_item_id) ON DELETE CASCADE,
    FOREIGN KEY (customer_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (item_id) REFERENCES menu_items(item_id) ON DELETE CASCADE
);

CREATE TABLE promotions (
    promo_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    code            TEXT NOT NULL UNIQUE,
    description     TEXT,
    discount_type   TEXT NOT NULL CHECK (discount_type IN ('percentage','fixed_amount')),
    discount_value  NUMERIC NOT NULL,
    min_order_value NUMERIC DEFAULT 0,
    max_discount    NUMERIC,
    valid_from      TEXT NOT NULL,
    valid_until     TEXT NOT NULL,
    usage_limit     INTEGER,
    restaurant_id   INTEGER,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(restaurant_id) ON DELETE CASCADE
);

CREATE TABLE order_promotions (
    order_promo_id  INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id        INTEGER NOT NULL,
    promo_id        INTEGER NOT NULL,
    discount_applied NUMERIC NOT NULL,
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (promo_id) REFERENCES promotions(promo_id) ON DELETE RESTRICT
);

CREATE INDEX idx_addresses_user            ON addresses(user_id);
CREATE INDEX idx_restaurants_owner         ON restaurants(owner_id);
CREATE INDEX idx_restaurants_city          ON restaurants(city);
CREATE INDEX idx_restaurant_hours_rest     ON restaurant_hours(restaurant_id);
CREATE INDEX idx_menu_categories_rest      ON menu_categories(restaurant_id);
CREATE INDEX idx_menu_items_rest           ON menu_items(restaurant_id);
CREATE INDEX idx_menu_items_category       ON menu_items(category_id);
CREATE INDEX idx_modifier_groups_item      ON modifier_groups(item_id);
CREATE INDEX idx_modifier_options_group    ON modifier_options(modifier_group_id);
CREATE INDEX idx_drivers_user              ON drivers(user_id);
CREATE INDEX idx_drivers_available         ON drivers(is_available);
CREATE INDEX idx_orders_customer           ON orders(customer_id);
CREATE INDEX idx_orders_restaurant         ON orders(restaurant_id);
CREATE INDEX idx_orders_driver             ON orders(driver_id);
CREATE INDEX idx_orders_status             ON orders(status);
CREATE INDEX idx_order_items_order         ON order_items(order_id);
CREATE INDEX idx_order_items_item          ON order_items(item_id);
CREATE INDEX idx_order_item_modifiers_item ON order_item_modifiers(order_item_id);
CREATE INDEX idx_order_status_hist_order   ON order_status_history(order_id);
CREATE INDEX idx_payments_user             ON payments(user_id);
CREATE INDEX idx_restaurant_ratings_rest   ON restaurant_ratings(restaurant_id);
CREATE INDEX idx_driver_ratings_driver     ON driver_ratings(driver_id);
CREATE INDEX idx_menu_item_ratings_item    ON menu_item_ratings(item_id);
CREATE INDEX idx_order_promotions_order    ON order_promotions(order_id);
CREATE INDEX idx_promotions_restaurant     ON promotions(restaurant_id);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE user_id = NEW.user_id;
END;

CREATE TRIGGER trg_restaurants_updated_at
AFTER UPDATE ON restaurants
BEGIN
    UPDATE restaurants SET updated_at = datetime('now') WHERE restaurant_id = NEW.restaurant_id;
END;

CREATE TRIGGER trg_menu_items_updated_at
AFTER UPDATE ON menu_items
BEGIN
    UPDATE menu_items SET updated_at = datetime('now') WHERE item_id = NEW.item_id;
END;

CREATE TRIGGER trg_order_status_history
AFTER UPDATE OF status ON orders
WHEN NEW.status <> OLD.status
BEGIN
    INSERT INTO order_status_history (order_id, status, changed_at)
    VALUES (NEW.order_id, NEW.status, datetime('now'));
END;
`
