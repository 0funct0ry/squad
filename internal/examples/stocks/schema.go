package stocks

const Schema = `-- Stock trading platform schema

PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    username            TEXT NOT NULL UNIQUE,
    email               TEXT NOT NULL UNIQUE,
    password_hash       TEXT NOT NULL,
    full_name           TEXT NOT NULL,
    phone               TEXT,
    kyc_status          TEXT NOT NULL DEFAULT 'PENDING'
                            CHECK (kyc_status IN ('PENDING','VERIFIED','REJECTED')),
    status              TEXT NOT NULL DEFAULT 'ACTIVE'
                            CHECK (status IN ('ACTIVE','SUSPENDED','CLOSED')),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE accounts (
    account_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id             INTEGER NOT NULL,
    account_number      TEXT NOT NULL UNIQUE,
    account_type        TEXT NOT NULL DEFAULT 'CASH'
                            CHECK (account_type IN ('CASH','MARGIN','RETIREMENT')),
    base_currency        TEXT NOT NULL DEFAULT 'USD',
    status              TEXT NOT NULL DEFAULT 'ACTIVE'
                            CHECK (status IN ('ACTIVE','FROZEN','CLOSED')),
    margin_enabled      INTEGER NOT NULL DEFAULT 0 CHECK (margin_enabled IN (0,1)),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_accounts_user ON accounts(user_id);

CREATE TABLE account_balances (
    balance_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id          INTEGER NOT NULL,
    currency            TEXT NOT NULL,
    cash_balance        NUMERIC NOT NULL DEFAULT 0,
    available_balance   NUMERIC NOT NULL DEFAULT 0,
    reserved_balance    NUMERIC NOT NULL DEFAULT 0,
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (account_id) REFERENCES accounts(account_id) ON DELETE CASCADE,
    UNIQUE (account_id, currency)
);

CREATE TABLE cash_ledger (
    ledger_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id          INTEGER NOT NULL,
    entry_type          TEXT NOT NULL
                            CHECK (entry_type IN ('DEPOSIT','WITHDRAWAL','TRADE_SETTLE',
                                                   'FEE','DIVIDEND','INTEREST','ADJUSTMENT')),
    currency            TEXT NOT NULL,
    amount              NUMERIC NOT NULL,
    balance_after       NUMERIC NOT NULL,
    reference_type      TEXT,
    reference_id        INTEGER,
    description         TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (account_id) REFERENCES accounts(account_id) ON DELETE CASCADE
);

CREATE INDEX idx_cash_ledger_account ON cash_ledger(account_id, created_at);

CREATE TABLE exchanges (
    exchange_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    mic_code            TEXT NOT NULL UNIQUE,
    name                TEXT NOT NULL,
    country             TEXT NOT NULL,
    timezone            TEXT NOT NULL DEFAULT 'UTC',
    open_time           TEXT,
    close_time          TEXT
);

CREATE TABLE instruments (
    instrument_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol              TEXT NOT NULL,
    exchange_id         INTEGER NOT NULL,
    isin                TEXT,
    cusip               TEXT,
    instrument_type     TEXT NOT NULL DEFAULT 'EQUITY'
                            CHECK (instrument_type IN ('EQUITY','ETF','BOND','OPTION','FUTURE','INDEX')),
    name                TEXT NOT NULL,
    currency            TEXT NOT NULL DEFAULT 'USD',
    tick_size           NUMERIC NOT NULL DEFAULT 0.01,
    lot_size            INTEGER NOT NULL DEFAULT 1,
    status              TEXT NOT NULL DEFAULT 'ACTIVE'
                            CHECK (status IN ('ACTIVE','HALTED','DELISTED')),
    listed_date         TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (exchange_id) REFERENCES exchanges(exchange_id),
    UNIQUE (symbol, exchange_id)
);

CREATE INDEX idx_instruments_symbol ON instruments(symbol);

CREATE TABLE market_data_bars (
    bar_id              INTEGER PRIMARY KEY AUTOINCREMENT,
    instrument_id       INTEGER NOT NULL,
    interval            TEXT NOT NULL
                            CHECK (interval IN ('1m','5m','15m','1h','1d')),
    bar_time            TEXT NOT NULL,
    open_price          NUMERIC NOT NULL,
    high_price          NUMERIC NOT NULL,
    low_price           NUMERIC NOT NULL,
    close_price         NUMERIC NOT NULL,
    volume              INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (instrument_id) REFERENCES instruments(instrument_id) ON DELETE CASCADE,
    UNIQUE (instrument_id, interval, bar_time)
);

CREATE INDEX idx_bars_instrument_time ON market_data_bars(instrument_id, interval, bar_time);

CREATE TABLE market_quotes (
    instrument_id       INTEGER PRIMARY KEY,
    bid_price           NUMERIC,
    bid_size            INTEGER,
    ask_price           NUMERIC,
    ask_size            INTEGER,
    last_price          NUMERIC,
    last_size           INTEGER,
    last_trade_time     TEXT,
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (instrument_id) REFERENCES instruments(instrument_id) ON DELETE CASCADE
);

CREATE TABLE market_ticks (
    tick_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    instrument_id       INTEGER NOT NULL,
    price               NUMERIC NOT NULL,
    size                INTEGER NOT NULL,
    tick_time           TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (instrument_id) REFERENCES instruments(instrument_id) ON DELETE CASCADE
);

CREATE INDEX idx_ticks_instrument_time ON market_ticks(instrument_id, tick_time);

CREATE TABLE order_book_entries (
    book_entry_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    instrument_id       INTEGER NOT NULL,
    order_id            INTEGER NOT NULL,
    side                TEXT NOT NULL CHECK (side IN ('BUY','SELL')),
    price               NUMERIC,
    quantity_remaining  INTEGER NOT NULL CHECK (quantity_remaining >= 0),
    priority_seq        INTEGER NOT NULL,
    entered_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (instrument_id) REFERENCES instruments(instrument_id) ON DELETE CASCADE,
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE
);

CREATE INDEX idx_book_instrument_side_price
    ON order_book_entries(instrument_id, side, price, priority_seq);

CREATE TABLE orders (
    order_id            INTEGER PRIMARY KEY AUTOINCREMENT,
    order_ref           TEXT NOT NULL UNIQUE,
    account_id           INTEGER NOT NULL,
    instrument_id        INTEGER NOT NULL,
    side                 TEXT NOT NULL CHECK (side IN ('BUY','SELL')),
    order_type           TEXT NOT NULL
                            CHECK (order_type IN ('MARKET','LIMIT','STOP','STOP_LIMIT')),
    time_in_force         TEXT NOT NULL DEFAULT 'DAY'
                            CHECK (time_in_force IN ('DAY','GTC','IOC','FOK')),
    quantity             INTEGER NOT NULL CHECK (quantity > 0),
    filled_quantity       INTEGER NOT NULL DEFAULT 0 CHECK (filled_quantity >= 0),
    limit_price           NUMERIC,
    stop_price            NUMERIC,
    avg_fill_price         NUMERIC,
    status                TEXT NOT NULL DEFAULT 'NEW'
                            CHECK (status IN ('NEW','PARTIALLY_FILLED','FILLED',
                                               'CANCELLED','REJECTED','EXPIRED')),
    reject_reason          TEXT,
    parent_order_id         INTEGER,
    submitted_at            TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at              TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at               TEXT,
    FOREIGN KEY (account_id) REFERENCES accounts(account_id),
    FOREIGN KEY (instrument_id) REFERENCES instruments(instrument_id),
    FOREIGN KEY (parent_order_id) REFERENCES orders(order_id),
    CHECK (
        (order_type IN ('LIMIT','STOP_LIMIT') AND limit_price IS NOT NULL)
        OR (order_type IN ('MARKET','STOP'))
    ),
    CHECK (
        (order_type IN ('STOP','STOP_LIMIT') AND stop_price IS NOT NULL)
        OR (order_type IN ('MARKET','LIMIT'))
    )
);

CREATE INDEX idx_orders_account ON orders(account_id, status);
CREATE INDEX idx_orders_instrument_status ON orders(instrument_id, status);

CREATE TABLE order_events (
    event_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id              INTEGER NOT NULL,
    event_type             TEXT NOT NULL
                            CHECK (event_type IN ('NEW','ACKNOWLEDGED','PARTIAL_FILL','FILL',
                                                   'CANCEL_REQUEST','CANCELLED','REJECTED',
                                                   'REPLACED','EXPIRED')),
    event_data              TEXT,
    event_time               TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE
);

CREATE INDEX idx_order_events_order ON order_events(order_id, event_time);

CREATE TABLE trades (
    trade_id              INTEGER PRIMARY KEY AUTOINCREMENT,
    trade_ref               TEXT NOT NULL UNIQUE,
    instrument_id             INTEGER NOT NULL,
    buy_order_id               INTEGER NOT NULL,
    sell_order_id                INTEGER NOT NULL,
    price                        NUMERIC NOT NULL CHECK (price > 0),
    quantity                     INTEGER NOT NULL CHECK (quantity > 0),
    trade_time                    TEXT NOT NULL DEFAULT (datetime('now')),
    trade_status                   TEXT NOT NULL DEFAULT 'EXECUTED'
                            CHECK (trade_status IN ('EXECUTED','REVERSED','BUSTED')),
    FOREIGN KEY (instrument_id) REFERENCES instruments(instrument_id),
    FOREIGN KEY (buy_order_id) REFERENCES orders(order_id),
    FOREIGN KEY (sell_order_id) REFERENCES orders(order_id)
);

CREATE INDEX idx_trades_instrument_time ON trades(instrument_id, trade_time);
CREATE INDEX idx_trades_buy_order ON trades(buy_order_id);
CREATE INDEX idx_trades_sell_order ON trades(sell_order_id);

CREATE TABLE trade_executions (
    execution_id            INTEGER PRIMARY KEY AUTOINCREMENT,
    trade_id                  INTEGER NOT NULL,
    order_id                    INTEGER NOT NULL,
    account_id                   INTEGER NOT NULL,
    side                          TEXT NOT NULL CHECK (side IN ('BUY','SELL')),
    price                          NUMERIC NOT NULL,
    quantity                       INTEGER NOT NULL,
    commission                      NUMERIC NOT NULL DEFAULT 0,
    fees                              NUMERIC NOT NULL DEFAULT 0,
    net_amount                        NUMERIC NOT NULL,
    executed_at                        TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (trade_id) REFERENCES trades(trade_id) ON DELETE CASCADE,
    FOREIGN KEY (order_id) REFERENCES orders(order_id),
    FOREIGN KEY (account_id) REFERENCES accounts(account_id)
);

CREATE INDEX idx_exec_account ON trade_executions(account_id, executed_at);
CREATE INDEX idx_exec_trade ON trade_executions(trade_id);

CREATE TABLE settlements (
    settlement_id            INTEGER PRIMARY KEY AUTOINCREMENT,
    trade_id                   INTEGER NOT NULL,
    account_id                   INTEGER NOT NULL,
    settlement_type                TEXT NOT NULL CHECK (settlement_type IN ('CASH','SECURITIES')),
    amount                          NUMERIC NOT NULL,
    quantity                         INTEGER,
    currency                          TEXT NOT NULL DEFAULT 'USD',
    settlement_date                    TEXT NOT NULL,
    status                               TEXT NOT NULL DEFAULT 'PENDING'
                            CHECK (status IN ('PENDING','SETTLED','FAILED','CANCELLED')),
    settled_at                            TEXT,
    failure_reason                          TEXT,
    FOREIGN KEY (trade_id) REFERENCES trades(trade_id),
    FOREIGN KEY (account_id) REFERENCES accounts(account_id)
);

CREATE INDEX idx_settlements_account_status ON settlements(account_id, status);
CREATE INDEX idx_settlements_date ON settlements(settlement_date);

CREATE TABLE portfolios (
    portfolio_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id                 INTEGER NOT NULL UNIQUE,
    name                         TEXT NOT NULL DEFAULT 'Default Portfolio',
    created_at                    TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (account_id) REFERENCES accounts(account_id) ON DELETE CASCADE
);

CREATE TABLE positions (
    position_id               INTEGER PRIMARY KEY AUTOINCREMENT,
    portfolio_id                 INTEGER NOT NULL,
    instrument_id                  INTEGER NOT NULL,
    quantity                         INTEGER NOT NULL DEFAULT 0,
    avg_cost_price                    NUMERIC NOT NULL DEFAULT 0,
    realized_pnl                        NUMERIC NOT NULL DEFAULT 0,
    updated_at                            TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (portfolio_id) REFERENCES portfolios(portfolio_id) ON DELETE CASCADE,
    FOREIGN KEY (instrument_id) REFERENCES instruments(instrument_id),
    UNIQUE (portfolio_id, instrument_id)
);

CREATE INDEX idx_positions_portfolio ON positions(portfolio_id);

CREATE TABLE position_snapshots (
    snapshot_id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    portfolio_id                   INTEGER NOT NULL,
    instrument_id                    INTEGER NOT NULL,
    snapshot_date                      TEXT NOT NULL,
    quantity                             INTEGER NOT NULL,
    avg_cost_price                         NUMERIC NOT NULL,
    market_price                             NUMERIC NOT NULL,
    market_value                               NUMERIC NOT NULL,
    unrealized_pnl                               NUMERIC NOT NULL,
    FOREIGN KEY (portfolio_id) REFERENCES portfolios(portfolio_id) ON DELETE CASCADE,
    FOREIGN KEY (instrument_id) REFERENCES instruments(instrument_id),
    UNIQUE (portfolio_id, instrument_id, snapshot_date)
);

CREATE INDEX idx_snapshots_date ON position_snapshots(snapshot_date);

CREATE TABLE corporate_actions (
    action_id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    instrument_id                 INTEGER NOT NULL,
    action_type                     TEXT NOT NULL
                            CHECK (action_type IN ('DIVIDEND','SPLIT','MERGER','SPINOFF')),
    ex_date                          TEXT NOT NULL,
    record_date                        TEXT,
    pay_date                             TEXT,
    ratio                                  NUMERIC,
    cash_amount                              NUMERIC,
    description                                TEXT,
    FOREIGN KEY (instrument_id) REFERENCES instruments(instrument_id)
);

CREATE INDEX idx_corp_actions_instrument ON corporate_actions(instrument_id, ex_date);

CREATE TABLE audit_log (
    audit_id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                        INTEGER,
    entity_type                      TEXT NOT NULL,
    entity_id                          INTEGER NOT NULL,
    action                                TEXT NOT NULL,
    old_value                              TEXT,
    new_value                                TEXT,
    created_at                                 TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE user_id = NEW.user_id;
END;

CREATE TRIGGER trg_orders_updated_at
AFTER UPDATE ON orders
BEGIN
    UPDATE orders SET updated_at = datetime('now') WHERE order_id = NEW.order_id;
END;

CREATE TRIGGER trg_account_create_portfolio
AFTER INSERT ON accounts
BEGIN
    INSERT INTO portfolios (account_id, name)
    VALUES (NEW.account_id, 'Default Portfolio');
END;

CREATE VIEW v_open_orders AS
SELECT
    o.order_id,
    o.order_ref,
    o.account_id,
    o.instrument_id,
    i.symbol,
    o.side,
    o.order_type,
    o.quantity,
    o.filled_quantity,
    (o.quantity - o.filled_quantity) AS remaining_quantity,
    o.limit_price,
    o.status,
    o.submitted_at
FROM orders o
JOIN instruments i ON i.instrument_id = o.instrument_id
WHERE o.status IN ('NEW','PARTIALLY_FILLED');

CREATE VIEW v_best_bid_offer AS
SELECT
    instrument_id,
    MAX(CASE WHEN side = 'BUY' THEN price END)  AS best_bid,
    MIN(CASE WHEN side = 'SELL' THEN price END) AS best_ask
FROM order_book_entries
WHERE quantity_remaining > 0
GROUP BY instrument_id;

CREATE VIEW v_portfolio_value AS
SELECT
    p.portfolio_id,
    p.instrument_id,
    i.symbol,
    p.quantity,
    p.avg_cost_price,
    q.last_price,
    (p.quantity * q.last_price) AS market_value,
    (p.quantity * (q.last_price - p.avg_cost_price)) AS unrealized_pnl
FROM positions p
JOIN instruments i ON i.instrument_id = p.instrument_id
LEFT JOIN market_quotes q ON q.instrument_id = p.instrument_id
WHERE p.quantity <> 0;
`
