package airline

const Schema = `-- Airline reservation system schema
PRAGMA foreign_keys = ON;

CREATE TABLE airports (
    airport_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    iata_code       TEXT NOT NULL UNIQUE CHECK (length(iata_code) = 3),
    icao_code       TEXT UNIQUE CHECK (icao_code IS NULL OR length(icao_code) = 4),
    name            TEXT NOT NULL,
    city            TEXT NOT NULL,
    country         TEXT NOT NULL,
    timezone        TEXT NOT NULL,
    latitude        REAL,
    longitude       REAL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE aircraft_types (
    aircraft_type_id  INTEGER PRIMARY KEY AUTOINCREMENT,
    manufacturer      TEXT NOT NULL,
    model             TEXT NOT NULL,
    total_seats       INTEGER NOT NULL CHECK (total_seats > 0),
    range_km          INTEGER,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (manufacturer, model)
);

CREATE TABLE seat_templates (
    seat_template_id  INTEGER PRIMARY KEY AUTOINCREMENT,
    aircraft_type_id  INTEGER NOT NULL REFERENCES aircraft_types(aircraft_type_id) ON DELETE CASCADE,
    seat_number       TEXT NOT NULL,
    cabin_class       TEXT NOT NULL CHECK (cabin_class IN ('ECONOMY','PREMIUM_ECONOMY','BUSINESS','FIRST')),
    seat_type         TEXT NOT NULL DEFAULT 'STANDARD' CHECK (seat_type IN ('STANDARD','EXIT_ROW','EXTRA_LEGROOM','BULKHEAD')),
    is_window         INTEGER NOT NULL DEFAULT 0 CHECK (is_window IN (0,1)),
    is_aisle          INTEGER NOT NULL DEFAULT 0 CHECK (is_aisle IN (0,1)),
    UNIQUE (aircraft_type_id, seat_number)
);

CREATE TABLE fare_classes (
    fare_class_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    code              TEXT NOT NULL UNIQUE,
    cabin_class       TEXT NOT NULL CHECK (cabin_class IN ('ECONOMY','PREMIUM_ECONOMY','BUSINESS','FIRST')),
    fare_name         TEXT NOT NULL,
    refundable        INTEGER NOT NULL DEFAULT 0 CHECK (refundable IN (0,1)),
    checked_bag_allowance_kg INTEGER NOT NULL DEFAULT 0,
    change_fee        NUMERIC NOT NULL DEFAULT 0
);

CREATE TABLE aircraft (
    aircraft_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    tail_number       TEXT NOT NULL UNIQUE,
    aircraft_type_id  INTEGER NOT NULL REFERENCES aircraft_types(aircraft_type_id),
    manufactured_year INTEGER,
    status            TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','MAINTENANCE','RETIRED')),
    last_maintenance_date TEXT,
    created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE routes (
    route_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    origin_airport_id INTEGER NOT NULL REFERENCES airports(airport_id),
    destination_airport_id INTEGER NOT NULL REFERENCES airports(airport_id),
    distance_km       INTEGER,
    typical_duration_minutes INTEGER,
    UNIQUE (origin_airport_id, destination_airport_id),
    CHECK (origin_airport_id <> destination_airport_id)
);

CREATE TABLE flight_schedules (
    flight_schedule_id INTEGER PRIMARY KEY AUTOINCREMENT,
    flight_number      TEXT NOT NULL,
    route_id           INTEGER NOT NULL REFERENCES routes(route_id),
    aircraft_type_id   INTEGER NOT NULL REFERENCES aircraft_types(aircraft_type_id),
    scheduled_departure_time TEXT NOT NULL,
    scheduled_arrival_time   TEXT NOT NULL,
    days_of_week       TEXT NOT NULL,
    effective_from     TEXT NOT NULL,
    effective_to       TEXT,
    UNIQUE (flight_number, route_id, scheduled_departure_time)
);

CREATE TABLE flights (
    flight_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    flight_schedule_id INTEGER NOT NULL REFERENCES flight_schedules(flight_schedule_id),
    aircraft_id        INTEGER REFERENCES aircraft(aircraft_id),
    flight_date        TEXT NOT NULL,
    departure_datetime_utc TEXT NOT NULL,
    arrival_datetime_utc   TEXT NOT NULL,
    gate                TEXT,
    status              TEXT NOT NULL DEFAULT 'SCHEDULED'
                         CHECK (status IN ('SCHEDULED','BOARDING','DEPARTED','IN_AIR','LANDED','ARRIVED','CANCELLED','DELAYED')),
    delay_minutes       INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (flight_schedule_id, flight_date)
);

CREATE INDEX idx_flights_date ON flights(flight_date);
CREATE INDEX idx_flights_status ON flights(status);

CREATE TABLE flight_fare_inventory (
    flight_fare_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    flight_id          INTEGER NOT NULL REFERENCES flights(flight_id) ON DELETE CASCADE,
    fare_class_id      INTEGER NOT NULL REFERENCES fare_classes(fare_class_id),
    base_price         NUMERIC NOT NULL CHECK (base_price >= 0),
    currency           TEXT NOT NULL DEFAULT 'USD',
    seats_allocated    INTEGER NOT NULL CHECK (seats_allocated >= 0),
    seats_sold         INTEGER NOT NULL DEFAULT 0 CHECK (seats_sold >= 0),
    UNIQUE (flight_id, fare_class_id),
    CHECK (seats_sold <= seats_allocated)
);

CREATE TABLE flight_seats (
    flight_seat_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    flight_id          INTEGER NOT NULL REFERENCES flights(flight_id) ON DELETE CASCADE,
    seat_template_id   INTEGER NOT NULL REFERENCES seat_templates(seat_template_id),
    is_available       INTEGER NOT NULL DEFAULT 1 CHECK (is_available IN (0,1)),
    UNIQUE (flight_id, seat_template_id)
);

CREATE INDEX idx_flight_seats_flight ON flight_seats(flight_id);

CREATE TABLE passengers (
    passenger_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name         TEXT NOT NULL,
    last_name          TEXT NOT NULL,
    date_of_birth      TEXT,
    gender             TEXT CHECK (gender IN ('MALE','FEMALE','OTHER','UNSPECIFIED')),
    email              TEXT UNIQUE,
    phone              TEXT,
    passport_number    TEXT,
    nationality        TEXT,
    created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE loyalty_programs (
    loyalty_program_id INTEGER PRIMARY KEY AUTOINCREMENT,
    program_name        TEXT NOT NULL UNIQUE,
    description          TEXT
);

CREATE TABLE loyalty_tiers (
    loyalty_tier_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    loyalty_program_id  INTEGER NOT NULL REFERENCES loyalty_programs(loyalty_program_id) ON DELETE CASCADE,
    tier_name           TEXT NOT NULL,
    min_points_required INTEGER NOT NULL DEFAULT 0,
    benefits            TEXT,
    UNIQUE (loyalty_program_id, tier_name)
);

CREATE TABLE loyalty_accounts (
    loyalty_account_id  INTEGER PRIMARY KEY AUTOINCREMENT,
    passenger_id         INTEGER NOT NULL REFERENCES passengers(passenger_id) ON DELETE CASCADE,
    loyalty_program_id   INTEGER NOT NULL REFERENCES loyalty_programs(loyalty_program_id),
    membership_number     TEXT NOT NULL UNIQUE,
    current_tier_id       INTEGER REFERENCES loyalty_tiers(loyalty_tier_id),
    points_balance        INTEGER NOT NULL DEFAULT 0 CHECK (points_balance >= 0),
    enrolled_at           TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (passenger_id, loyalty_program_id)
);

CREATE TABLE loyalty_transactions (
    loyalty_txn_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    loyalty_account_id   INTEGER NOT NULL REFERENCES loyalty_accounts(loyalty_account_id) ON DELETE CASCADE,
    txn_type             TEXT NOT NULL CHECK (txn_type IN ('EARN','REDEEM','ADJUST','EXPIRE')),
    points               INTEGER NOT NULL,
    booking_id           INTEGER REFERENCES bookings(booking_id),
    description          TEXT,
    txn_date             TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE bookings (
    booking_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    pnr                  TEXT NOT NULL UNIQUE,
    booking_passenger_id INTEGER NOT NULL REFERENCES passengers(passenger_id),
    booking_status       TEXT NOT NULL DEFAULT 'CONFIRMED'
                          CHECK (booking_status IN ('CONFIRMED','PENDING','CANCELLED','COMPLETED')),
    booking_channel      TEXT DEFAULT 'WEBSITE' CHECK (booking_channel IN ('WEBSITE','MOBILE_APP','AGENT','KIOSK','PARTNER')),
    total_amount         NUMERIC NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    currency             TEXT NOT NULL DEFAULT 'USD',
    created_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_bookings_pnr ON bookings(pnr);

CREATE TABLE tickets (
    ticket_id            INTEGER PRIMARY KEY AUTOINCREMENT,
    ticket_number        TEXT NOT NULL UNIQUE,
    booking_id           INTEGER NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    passenger_id         INTEGER NOT NULL REFERENCES passengers(passenger_id),
    flight_id            INTEGER NOT NULL REFERENCES flights(flight_id),
    fare_class_id        INTEGER NOT NULL REFERENCES fare_classes(fare_class_id),
    flight_seat_id        INTEGER REFERENCES flight_seats(flight_seat_id),
    fare_amount           NUMERIC NOT NULL CHECK (fare_amount >= 0),
    taxes_fees            NUMERIC NOT NULL DEFAULT 0 CHECK (taxes_fees >= 0),
    ticket_status         TEXT NOT NULL DEFAULT 'ISSUED'
                           CHECK (ticket_status IN ('ISSUED','CHECKED_IN','BOARDED','FLOWN','CANCELLED','REFUNDED','NO_SHOW')),
    checked_in_at         TEXT,
    baggage_count         INTEGER NOT NULL DEFAULT 0,
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (flight_id, flight_seat_id)
);

CREATE INDEX idx_tickets_booking ON tickets(booking_id);
CREATE INDEX idx_tickets_flight ON tickets(flight_id);
CREATE INDEX idx_tickets_passenger ON tickets(passenger_id);

CREATE TABLE payments (
    payment_id            INTEGER PRIMARY KEY AUTOINCREMENT,
    booking_id            INTEGER NOT NULL REFERENCES bookings(booking_id) ON DELETE CASCADE,
    amount                NUMERIC NOT NULL CHECK (amount >= 0),
    currency              TEXT NOT NULL DEFAULT 'USD',
    payment_method         TEXT NOT NULL CHECK (payment_method IN ('CREDIT_CARD','DEBIT_CARD','PAYPAL','LOYALTY_POINTS','BANK_TRANSFER','VOUCHER')),
    payment_status         TEXT NOT NULL DEFAULT 'PENDING'
                           CHECK (payment_status IN ('PENDING','AUTHORIZED','CAPTURED','FAILED','REFUNDED')),
    transaction_reference  TEXT,
    paid_at                TEXT,
    created_at             TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_payments_booking ON payments(booking_id);

CREATE TABLE employees (
    employee_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    employee_code         TEXT NOT NULL UNIQUE,
    first_name            TEXT NOT NULL,
    last_name             TEXT NOT NULL,
    role                  TEXT NOT NULL CHECK (role IN ('PILOT','FIRST_OFFICER','FLIGHT_ATTENDANT','PURSER','ENGINEER')),
    date_of_birth         TEXT,
    hire_date             TEXT NOT NULL,
    base_airport_id       INTEGER REFERENCES airports(airport_id),
    status                TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','ON_LEAVE','SUSPENDED','TERMINATED')),
    created_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE crew_qualifications (
    qualification_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    employee_id           INTEGER NOT NULL REFERENCES employees(employee_id) ON DELETE CASCADE,
    aircraft_type_id       INTEGER NOT NULL REFERENCES aircraft_types(aircraft_type_id),
    qualification_type     TEXT NOT NULL CHECK (qualification_type IN ('TYPE_RATING','CABIN_CREW_CERT','INSTRUCTOR')),
    certified_on           TEXT NOT NULL,
    expires_on             TEXT,
    UNIQUE (employee_id, aircraft_type_id, qualification_type)
);

CREATE TABLE crew_assignments (
    crew_assignment_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    flight_id              INTEGER NOT NULL REFERENCES flights(flight_id) ON DELETE CASCADE,
    employee_id            INTEGER NOT NULL REFERENCES employees(employee_id),
    duty_role              TEXT NOT NULL CHECK (duty_role IN ('CAPTAIN','FIRST_OFFICER','PURSER','FLIGHT_ATTENDANT','RELIEF_PILOT')),
    report_time_utc        TEXT NOT NULL,
    release_time_utc       TEXT,
    assignment_status       TEXT NOT NULL DEFAULT 'ASSIGNED'
                            CHECK (assignment_status IN ('ASSIGNED','CONFIRMED','COMPLETED','CANCELLED','NO_SHOW')),
    created_at              TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (flight_id, employee_id)
);

CREATE INDEX idx_crew_assignments_flight ON crew_assignments(flight_id);
CREATE INDEX idx_crew_assignments_employee ON crew_assignments(employee_id);

CREATE TABLE crew_duty_logs (
    duty_log_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    employee_id             INTEGER NOT NULL REFERENCES employees(employee_id) ON DELETE CASCADE,
    duty_date               TEXT NOT NULL,
    duty_start_utc           TEXT NOT NULL,
    duty_end_utc             TEXT,
    flight_hours             REAL NOT NULL DEFAULT 0,
    rest_hours_before         REAL,
    notes                    TEXT
);

CREATE INDEX idx_crew_duty_logs_employee ON crew_duty_logs(employee_id, duty_date);

CREATE VIEW v_flight_fare_availability AS
SELECT
    ffi.flight_id,
    f.flight_date,
    fc.code AS fare_code,
    fc.cabin_class,
    ffi.seats_allocated,
    ffi.seats_sold,
    (ffi.seats_allocated - ffi.seats_sold) AS seats_remaining,
    ffi.base_price,
    ffi.currency
FROM flight_fare_inventory ffi
JOIN flights f ON f.flight_id = ffi.flight_id
JOIN fare_classes fc ON fc.fare_class_id = ffi.fare_class_id;

CREATE VIEW v_booking_summary AS
SELECT
    b.booking_id,
    b.pnr,
    b.booking_status,
    b.total_amount,
    b.currency,
    COUNT(DISTINCT t.passenger_id) AS passenger_count,
    COUNT(DISTINCT t.flight_id) AS flight_count
FROM bookings b
LEFT JOIN tickets t ON t.booking_id = b.booking_id
GROUP BY b.booking_id;

CREATE VIEW v_flight_crew_roster AS
SELECT
    ca.flight_id,
    e.employee_code,
    e.first_name || ' ' || e.last_name AS crew_name,
    ca.duty_role,
    ca.assignment_status
FROM crew_assignments ca
JOIN employees e ON e.employee_id = ca.employee_id;
`
