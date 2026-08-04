package ecommerce

const Schema = `PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id         INTEGER PRIMARY KEY,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    phone           TEXT,
    status          TEXT NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active','inactive','banned','pending_verification')),
    email_verified_at TEXT,
    last_login_at   TEXT,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    updated_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now'))
);

CREATE INDEX idx_users_email ON users(email);

CREATE TABLE roles (
    role_id     INTEGER PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT
);

CREATE TABLE user_roles (
    user_id     INTEGER NOT NULL,
    role_id     INTEGER NOT NULL,
    assigned_at TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(role_id) ON DELETE CASCADE
);

CREATE TABLE addresses (
    address_id      INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL,
    label           TEXT,
    recipient_name  TEXT NOT NULL,
    phone           TEXT,
    line1           TEXT NOT NULL,
    line2           TEXT,
    city            TEXT NOT NULL,
    state           TEXT,
    postal_code     TEXT NOT NULL,
    country_code    TEXT NOT NULL,
    is_default_shipping INTEGER NOT NULL DEFAULT 0 CHECK (is_default_shipping IN (0,1)),
    is_default_billing  INTEGER NOT NULL DEFAULT 0 CHECK (is_default_billing IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_addresses_user ON addresses(user_id);

CREATE TABLE categories (
    category_id     INTEGER PRIMARY KEY,
    parent_id       INTEGER,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    description     TEXT,
    display_order   INTEGER NOT NULL DEFAULT 0,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    FOREIGN KEY (parent_id) REFERENCES categories(category_id) ON DELETE SET NULL
);

CREATE INDEX idx_categories_parent ON categories(parent_id);

CREATE TABLE brands (
    brand_id    INTEGER PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    slug        TEXT NOT NULL UNIQUE,
    logo_url    TEXT,
    is_active   INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1))
);

CREATE TABLE suppliers (
    supplier_id     INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    contact_email   TEXT,
    contact_phone   TEXT,
    address         TEXT,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1))
);

CREATE TABLE products (
    product_id      INTEGER PRIMARY KEY,
    brand_id        INTEGER,
    supplier_id     INTEGER,
    sku_prefix      TEXT,
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft','active','archived')),
    is_taxable      INTEGER NOT NULL DEFAULT 1 CHECK (is_taxable IN (0,1)),
    weight_grams    INTEGER,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    updated_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (brand_id) REFERENCES brands(brand_id) ON DELETE SET NULL,
    FOREIGN KEY (supplier_id) REFERENCES suppliers(supplier_id) ON DELETE SET NULL
);

CREATE INDEX idx_products_brand ON products(brand_id);
CREATE INDEX idx_products_status ON products(status);

CREATE TABLE product_categories (
    product_id      INTEGER NOT NULL,
    category_id     INTEGER NOT NULL,
    PRIMARY KEY (product_id, category_id),
    FOREIGN KEY (product_id) REFERENCES products(product_id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(category_id) ON DELETE CASCADE
);

CREATE TABLE attributes (
    attribute_id    INTEGER PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE
);

CREATE TABLE attribute_values (
    attribute_value_id INTEGER PRIMARY KEY,
    attribute_id        INTEGER NOT NULL,
    value               TEXT NOT NULL,
    UNIQUE (attribute_id, value),
    FOREIGN KEY (attribute_id) REFERENCES attributes(attribute_id) ON DELETE CASCADE
);

CREATE TABLE product_variants (
    variant_id      INTEGER PRIMARY KEY,
    product_id      INTEGER NOT NULL,
    sku             TEXT NOT NULL UNIQUE,
    barcode         TEXT,
    price_cents     INTEGER NOT NULL CHECK (price_cents >= 0),
    compare_at_cents INTEGER CHECK (compare_at_cents >= 0),
    currency        TEXT NOT NULL DEFAULT 'USD',
    weight_grams    INTEGER,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    updated_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (product_id) REFERENCES products(product_id) ON DELETE CASCADE
);

CREATE INDEX idx_variants_product ON product_variants(product_id);

CREATE TABLE variant_attribute_values (
    variant_id          INTEGER NOT NULL,
    attribute_value_id  INTEGER NOT NULL,
    PRIMARY KEY (variant_id, attribute_value_id),
    FOREIGN KEY (variant_id) REFERENCES product_variants(variant_id) ON DELETE CASCADE,
    FOREIGN KEY (attribute_value_id) REFERENCES attribute_values(attribute_value_id) ON DELETE CASCADE
);

CREATE TABLE product_images (
    image_id        INTEGER PRIMARY KEY,
    product_id      INTEGER NOT NULL,
    variant_id      INTEGER,
    url             TEXT NOT NULL,
    alt_text        TEXT,
    display_order   INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (product_id) REFERENCES products(product_id) ON DELETE CASCADE,
    FOREIGN KEY (variant_id) REFERENCES product_variants(variant_id) ON DELETE CASCADE
);

CREATE TABLE warehouses (
    warehouse_id    INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    address         TEXT,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1))
);

CREATE TABLE inventory (
    inventory_id        INTEGER PRIMARY KEY,
    variant_id          INTEGER NOT NULL,
    warehouse_id        INTEGER NOT NULL,
    quantity_on_hand    INTEGER NOT NULL DEFAULT 0 CHECK (quantity_on_hand >= 0),
    quantity_reserved   INTEGER NOT NULL DEFAULT 0 CHECK (quantity_reserved >= 0),
    reorder_threshold   INTEGER NOT NULL DEFAULT 0,
    updated_at          TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    UNIQUE (variant_id, warehouse_id),
    FOREIGN KEY (variant_id) REFERENCES product_variants(variant_id) ON DELETE CASCADE,
    FOREIGN KEY (warehouse_id) REFERENCES warehouses(warehouse_id) ON DELETE CASCADE
);

CREATE TABLE inventory_movements (
    movement_id     INTEGER PRIMARY KEY,
    inventory_id    INTEGER NOT NULL,
    change_qty      INTEGER NOT NULL,
    reason          TEXT NOT NULL
                        CHECK (reason IN ('purchase_order','sale','return','adjustment','damaged','transfer')),
    reference_type  TEXT,
    reference_id    INTEGER,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (inventory_id) REFERENCES inventory(inventory_id) ON DELETE CASCADE
);

CREATE TABLE carts (
    cart_id     INTEGER PRIMARY KEY,
    user_id     INTEGER,
    session_token TEXT,
    status      TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','converted','abandoned')),
    created_at  TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    updated_at  TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_carts_user ON carts(user_id);

CREATE TABLE cart_items (
    cart_item_id    INTEGER PRIMARY KEY,
    cart_id         INTEGER NOT NULL,
    variant_id      INTEGER NOT NULL,
    quantity        INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents INTEGER NOT NULL,
    added_at        TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    UNIQUE (cart_id, variant_id),
    FOREIGN KEY (cart_id) REFERENCES carts(cart_id) ON DELETE CASCADE,
    FOREIGN KEY (variant_id) REFERENCES product_variants(variant_id) ON DELETE CASCADE
);

CREATE TABLE coupons (
    coupon_id       INTEGER PRIMARY KEY,
    code            TEXT NOT NULL UNIQUE,
    description     TEXT,
    discount_type   TEXT NOT NULL CHECK (discount_type IN ('percentage','fixed_amount','free_shipping')),
    discount_value  INTEGER NOT NULL DEFAULT 0,
    min_order_cents INTEGER NOT NULL DEFAULT 0,
    max_redemptions INTEGER,
    times_redeemed  INTEGER NOT NULL DEFAULT 0,
    starts_at       TEXT,
    ends_at         TEXT,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1))
);

CREATE TABLE coupon_redemptions (
    redemption_id   INTEGER PRIMARY KEY,
    coupon_id       INTEGER NOT NULL,
    user_id         INTEGER,
    order_id        INTEGER NOT NULL,
    redeemed_at     TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (coupon_id) REFERENCES coupons(coupon_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE SET NULL,
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE
);

CREATE TABLE orders (
    order_id            INTEGER PRIMARY KEY,
    order_number        TEXT NOT NULL UNIQUE,
    user_id             INTEGER NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending'
                            CHECK (status IN (
                                'pending','awaiting_payment','paid','processing',
                                'shipped','delivered','cancelled','refunded','failed'
                            )),
    currency            TEXT NOT NULL DEFAULT 'USD',
    subtotal_cents      INTEGER NOT NULL DEFAULT 0,
    discount_cents      INTEGER NOT NULL DEFAULT 0,
    shipping_cents      INTEGER NOT NULL DEFAULT 0,
    tax_cents           INTEGER NOT NULL DEFAULT 0,
    total_cents         INTEGER NOT NULL DEFAULT 0,
    coupon_id           INTEGER,
    shipping_address_id INTEGER NOT NULL,
    billing_address_id  INTEGER NOT NULL,
    customer_notes      TEXT,
    placed_at           TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    updated_at          TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE RESTRICT,
    FOREIGN KEY (coupon_id) REFERENCES coupons(coupon_id) ON DELETE SET NULL,
    FOREIGN KEY (shipping_address_id) REFERENCES addresses(address_id) ON DELETE RESTRICT,
    FOREIGN KEY (billing_address_id) REFERENCES addresses(address_id) ON DELETE RESTRICT
);

CREATE INDEX idx_orders_user ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_placed_at ON orders(placed_at);

CREATE TABLE order_items (
    order_item_id       INTEGER PRIMARY KEY,
    order_id            INTEGER NOT NULL,
    variant_id          INTEGER NOT NULL,
    product_name_snapshot TEXT NOT NULL,
    sku_snapshot        TEXT NOT NULL,
    unit_price_cents    INTEGER NOT NULL,
    quantity            INTEGER NOT NULL CHECK (quantity > 0),
    line_total_cents    INTEGER NOT NULL,
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (variant_id) REFERENCES product_variants(variant_id) ON DELETE RESTRICT
);

CREATE INDEX idx_order_items_order ON order_items(order_id);

CREATE TABLE order_status_history (
    history_id      INTEGER PRIMARY KEY,
    order_id        INTEGER NOT NULL,
    status          TEXT NOT NULL,
    note            TEXT,
    changed_by_user_id INTEGER,
    changed_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (changed_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE TABLE payment_methods (
    payment_method_id   INTEGER PRIMARY KEY,
    user_id             INTEGER NOT NULL,
    type                TEXT NOT NULL CHECK (type IN ('credit_card','debit_card','paypal','bank_transfer','wallet')),
    provider            TEXT,
    provider_token      TEXT,
    last4               TEXT,
    expiry_month        INTEGER,
    expiry_year         INTEGER,
    is_default          INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)),
    created_at          TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE payments (
    payment_id          INTEGER PRIMARY KEY,
    order_id            INTEGER NOT NULL,
    payment_method_id   INTEGER,
    amount_cents        INTEGER NOT NULL CHECK (amount_cents >= 0),
    currency            TEXT NOT NULL DEFAULT 'USD',
    status              TEXT NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','authorized','captured','failed','refunded','partially_refunded','voided')),
    provider_transaction_id TEXT,
    processed_at        TEXT,
    created_at          TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (payment_method_id) REFERENCES payment_methods(payment_method_id) ON DELETE SET NULL
);

CREATE INDEX idx_payments_order ON payments(order_id);

CREATE TABLE refunds (
    refund_id       INTEGER PRIMARY KEY,
    payment_id      INTEGER NOT NULL,
    amount_cents    INTEGER NOT NULL CHECK (amount_cents >= 0),
    reason          TEXT,
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','completed','failed')),
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (payment_id) REFERENCES payments(payment_id) ON DELETE CASCADE
);

CREATE TABLE shipping_carriers (
    carrier_id      INTEGER PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    tracking_url_template TEXT
);

CREATE TABLE shipments (
    shipment_id     INTEGER PRIMARY KEY,
    order_id        INTEGER NOT NULL,
    carrier_id      INTEGER,
    warehouse_id    INTEGER,
    tracking_number TEXT,
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','packed','shipped','in_transit','delivered','returned','lost')),
    shipped_at      TEXT,
    delivered_at    TEXT,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (carrier_id) REFERENCES shipping_carriers(carrier_id) ON DELETE SET NULL,
    FOREIGN KEY (warehouse_id) REFERENCES warehouses(warehouse_id) ON DELETE SET NULL
);

CREATE INDEX idx_shipments_order ON shipments(order_id);

CREATE TABLE shipment_items (
    shipment_item_id    INTEGER PRIMARY KEY,
    shipment_id         INTEGER NOT NULL,
    order_item_id       INTEGER NOT NULL,
    quantity            INTEGER NOT NULL CHECK (quantity > 0),
    FOREIGN KEY (shipment_id) REFERENCES shipments(shipment_id) ON DELETE CASCADE,
    FOREIGN KEY (order_item_id) REFERENCES order_items(order_item_id) ON DELETE CASCADE
);

CREATE TABLE returns (
    return_id       INTEGER PRIMARY KEY,
    order_id        INTEGER NOT NULL,
    user_id         INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT 'requested'
                        CHECK (status IN ('requested','approved','rejected','received','refunded')),
    reason          TEXT,
    requested_at    TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    resolved_at     TEXT,
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE return_items (
    return_item_id  INTEGER PRIMARY KEY,
    return_id       INTEGER NOT NULL,
    order_item_id   INTEGER NOT NULL,
    quantity        INTEGER NOT NULL CHECK (quantity > 0),
    condition       TEXT CHECK (condition IN ('unopened','opened','damaged','defective')),
    FOREIGN KEY (return_id) REFERENCES returns(return_id) ON DELETE CASCADE,
    FOREIGN KEY (order_item_id) REFERENCES order_items(order_item_id) ON DELETE CASCADE
);

CREATE TABLE reviews (
    review_id       INTEGER PRIMARY KEY,
    product_id      INTEGER NOT NULL,
    user_id         INTEGER NOT NULL,
    order_item_id   INTEGER,
    rating          INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title           TEXT,
    body            TEXT,
    is_verified_purchase INTEGER NOT NULL DEFAULT 0 CHECK (is_verified_purchase IN (0,1)),
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','approved','rejected')),
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    UNIQUE (product_id, user_id, order_item_id),
    FOREIGN KEY (product_id) REFERENCES products(product_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (order_item_id) REFERENCES order_items(order_item_id) ON DELETE SET NULL
);

CREATE INDEX idx_reviews_product ON reviews(product_id);

CREATE TABLE wishlists (
    wishlist_id     INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL,
    name            TEXT NOT NULL DEFAULT 'My Wishlist',
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE wishlist_items (
    wishlist_id     INTEGER NOT NULL,
    variant_id      INTEGER NOT NULL,
    added_at        TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    PRIMARY KEY (wishlist_id, variant_id),
    FOREIGN KEY (wishlist_id) REFERENCES wishlists(wishlist_id) ON DELETE CASCADE,
    FOREIGN KEY (variant_id) REFERENCES product_variants(variant_id) ON DELETE CASCADE
);

CREATE TABLE tax_rates (
    tax_rate_id     INTEGER PRIMARY KEY,
    country_code    TEXT NOT NULL,
    state           TEXT,
    rate_percent    REAL NOT NULL CHECK (rate_percent >= 0),
    name            TEXT NOT NULL
);

CREATE INDEX idx_tax_rates_country_state ON tax_rates(country_code, state);

CREATE TABLE audit_logs (
    audit_id        INTEGER PRIMARY KEY,
    actor_user_id   INTEGER,
    action          TEXT NOT NULL,
    entity_type     TEXT NOT NULL,
    entity_id       INTEGER NOT NULL,
    details         TEXT,
    created_at      TEXT NOT NULL DEFAULT (STRFTIME('%Y-%m-%d %H:%M:%S','now')),
    FOREIGN KEY (actor_user_id) REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX idx_audit_entity ON audit_logs(entity_type, entity_id);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = STRFTIME('%Y-%m-%d %H:%M:%S','now') WHERE user_id = NEW.user_id;
END;

CREATE TRIGGER trg_products_updated_at
AFTER UPDATE ON products
BEGIN
    UPDATE products SET updated_at = STRFTIME('%Y-%m-%d %H:%M:%S','now') WHERE product_id = NEW.product_id;
END;

CREATE TRIGGER trg_variants_updated_at
AFTER UPDATE ON product_variants
BEGIN
    UPDATE product_variants SET updated_at = STRFTIME('%Y-%m-%d %H:%M:%S','now') WHERE variant_id = NEW.variant_id;
END;

CREATE TRIGGER trg_orders_updated_at
AFTER UPDATE ON orders
BEGIN
    UPDATE orders SET updated_at = STRFTIME('%Y-%m-%d %H:%M:%S','now') WHERE order_id = NEW.order_id;
END;

CREATE TRIGGER trg_carts_updated_at
AFTER UPDATE ON carts
BEGIN
    UPDATE carts SET updated_at = STRFTIME('%Y-%m-%d %H:%M:%S','now') WHERE cart_id = NEW.cart_id;
END;
`
