package library

const Schema = `-- Library Management System Schema

PRAGMA foreign_keys = ON;

CREATE TABLE branches (
    branch_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    address_line1   TEXT NOT NULL,
    address_line2   TEXT,
    city            TEXT NOT NULL,
    state           TEXT,
    postal_code     TEXT,
    country         TEXT NOT NULL DEFAULT 'USA',
    phone           TEXT,
    email           TEXT,
    opening_hours   TEXT,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE staff (
    staff_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    branch_id       INTEGER NOT NULL,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    email           TEXT NOT NULL UNIQUE,
    phone           TEXT,
    role            TEXT NOT NULL DEFAULT 'librarian'
                        CHECK (role IN ('librarian','branch_manager','admin','assistant')),
    hire_date       TEXT NOT NULL DEFAULT (date('now')),
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (branch_id) REFERENCES branches(branch_id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE TABLE members (
    member_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    home_branch_id      INTEGER NOT NULL,
    membership_number   TEXT NOT NULL UNIQUE,
    first_name          TEXT NOT NULL,
    last_name            TEXT NOT NULL,
    email               TEXT NOT NULL UNIQUE,
    phone               TEXT,
    address_line1       TEXT,
    address_line2       TEXT,
    city                TEXT,
    state               TEXT,
    postal_code         TEXT,
    date_of_birth       TEXT,
    membership_type     TEXT NOT NULL DEFAULT 'standard'
                            CHECK (membership_type IN ('standard','student','senior','staff','premium')),
    membership_status   TEXT NOT NULL DEFAULT 'active'
                            CHECK (membership_status IN ('active','suspended','expired','cancelled')),
    max_loans_allowed   INTEGER NOT NULL DEFAULT 5,
    joined_date         TEXT NOT NULL DEFAULT (date('now')),
    expiry_date         TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (home_branch_id) REFERENCES branches(branch_id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE TABLE authors (
    author_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    date_of_birth   TEXT,
    date_of_death   TEXT,
    nationality     TEXT,
    biography       TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (first_name, last_name, date_of_birth)
);

CREATE TABLE publishers (
    publisher_id    INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    address         TEXT,
    website         TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE categories (
    category_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    parent_category_id INTEGER,
    description     TEXT,
    FOREIGN KEY (parent_category_id) REFERENCES categories(category_id)
        ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE TABLE books (
    book_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    isbn                TEXT NOT NULL UNIQUE,
    title               TEXT NOT NULL,
    subtitle            TEXT,
    publisher_id        INTEGER,
    category_id         INTEGER,
    publication_year    INTEGER,
    edition             TEXT,
    language            TEXT NOT NULL DEFAULT 'English',
    page_count          INTEGER,
    description         TEXT,
    cover_image_url     TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (publisher_id) REFERENCES publishers(publisher_id)
        ON UPDATE CASCADE ON DELETE SET NULL,
    FOREIGN KEY (category_id) REFERENCES categories(category_id)
        ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE TABLE book_authors (
    book_id         INTEGER NOT NULL,
    author_id       INTEGER NOT NULL,
    author_order    INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (book_id, author_id),
    FOREIGN KEY (book_id) REFERENCES books(book_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (author_id) REFERENCES authors(author_id)
        ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE TABLE book_copies (
    copy_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id             INTEGER NOT NULL,
    branch_id           INTEGER NOT NULL,
    barcode             TEXT NOT NULL UNIQUE,
    acquisition_date    TEXT NOT NULL DEFAULT (date('now')),
    price               NUMERIC(10,2),
    shelf_location      TEXT,
    copy_condition      TEXT NOT NULL DEFAULT 'good'
                            CHECK (copy_condition IN ('new','good','fair','poor','damaged')),
    status              TEXT NOT NULL DEFAULT 'available'
                            CHECK (status IN ('available','on_loan','reserved','lost','withdrawn','in_repair','in_transit')),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (book_id) REFERENCES books(book_id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    FOREIGN KEY (branch_id) REFERENCES branches(branch_id)
        ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE TABLE loans (
    loan_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    copy_id             INTEGER NOT NULL,
    member_id           INTEGER NOT NULL,
    branch_id           INTEGER NOT NULL,
    issued_by_staff_id  INTEGER,
    checked_out_date    TEXT NOT NULL DEFAULT (datetime('now')),
    due_date            TEXT NOT NULL,
    returned_date       TEXT,
    returned_to_branch_id INTEGER,
    received_by_staff_id  INTEGER,
    renewal_count       INTEGER NOT NULL DEFAULT 0,
    status              TEXT NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active','returned','overdue','lost')),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (copy_id) REFERENCES book_copies(copy_id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    FOREIGN KEY (member_id) REFERENCES members(member_id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    FOREIGN KEY (branch_id) REFERENCES branches(branch_id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    FOREIGN KEY (issued_by_staff_id) REFERENCES staff(staff_id)
        ON UPDATE CASCADE ON DELETE SET NULL,
    FOREIGN KEY (returned_to_branch_id) REFERENCES branches(branch_id)
        ON UPDATE CASCADE ON DELETE SET NULL,
    FOREIGN KEY (received_by_staff_id) REFERENCES staff(staff_id)
        ON UPDATE CASCADE ON DELETE SET NULL,
    CHECK (returned_date IS NULL OR returned_date >= checked_out_date)
);

CREATE TABLE reservations (
    reservation_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id             INTEGER NOT NULL,
    member_id           INTEGER NOT NULL,
    pickup_branch_id    INTEGER NOT NULL,
    reservation_date    TEXT NOT NULL DEFAULT (datetime('now')),
    expiry_date         TEXT,
    status              TEXT NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','ready_for_pickup','fulfilled','cancelled','expired')),
    fulfilled_copy_id   INTEGER,
    fulfilled_loan_id   INTEGER,
    notified_at         TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (book_id) REFERENCES books(book_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (member_id) REFERENCES members(member_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (pickup_branch_id) REFERENCES branches(branch_id)
        ON UPDATE CASCADE ON DELETE RESTRICT,
    FOREIGN KEY (fulfilled_copy_id) REFERENCES book_copies(copy_id)
        ON UPDATE CASCADE ON DELETE SET NULL,
    FOREIGN KEY (fulfilled_loan_id) REFERENCES loans(loan_id)
        ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE TABLE fines (
    fine_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    member_id           INTEGER NOT NULL,
    loan_id             INTEGER,
    fine_type           TEXT NOT NULL DEFAULT 'overdue'
                            CHECK (fine_type IN ('overdue','lost_item','damaged_item','other')),
    amount              NUMERIC(10,2) NOT NULL CHECK (amount >= 0),
    amount_paid         NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (amount_paid >= 0),
    status              TEXT NOT NULL DEFAULT 'unpaid'
                            CHECK (status IN ('unpaid','partially_paid','paid','waived')),
    issued_date         TEXT NOT NULL DEFAULT (date('now')),
    paid_date           TEXT,
    waived_by_staff_id  INTEGER,
    notes               TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (member_id) REFERENCES members(member_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (loan_id) REFERENCES loans(loan_id)
        ON UPDATE CASCADE ON DELETE SET NULL,
    FOREIGN KEY (waived_by_staff_id) REFERENCES staff(staff_id)
        ON UPDATE CASCADE ON DELETE SET NULL,
    CHECK (amount_paid <= amount)
);

CREATE TABLE fine_payments (
    payment_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    fine_id             INTEGER NOT NULL,
    amount              NUMERIC(10,2) NOT NULL CHECK (amount > 0),
    payment_method      TEXT NOT NULL DEFAULT 'cash'
                            CHECK (payment_method IN ('cash','card','online','waiver')),
    received_by_staff_id INTEGER,
    payment_date        TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (fine_id) REFERENCES fines(fine_id)
        ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (received_by_staff_id) REFERENCES staff(staff_id)
        ON UPDATE CASCADE ON DELETE SET NULL
);

CREATE INDEX idx_staff_branch ON staff(branch_id);

CREATE INDEX idx_members_branch ON members(home_branch_id);
CREATE INDEX idx_members_status ON members(membership_status);

CREATE INDEX idx_books_title ON books(title);
CREATE INDEX idx_books_publisher ON books(publisher_id);
CREATE INDEX idx_books_category ON books(category_id);

CREATE INDEX idx_book_authors_author ON book_authors(author_id);

CREATE INDEX idx_copies_book ON book_copies(book_id);
CREATE INDEX idx_copies_branch ON book_copies(branch_id);
CREATE INDEX idx_copies_status ON book_copies(status);

CREATE INDEX idx_loans_member ON loans(member_id);
CREATE INDEX idx_loans_copy ON loans(copy_id);
CREATE INDEX idx_loans_branch ON loans(branch_id);
CREATE INDEX idx_loans_status ON loans(status);
CREATE INDEX idx_loans_due_date ON loans(due_date);

CREATE INDEX idx_reservations_book ON reservations(book_id);
CREATE INDEX idx_reservations_member ON reservations(member_id);
CREATE INDEX idx_reservations_branch ON reservations(pickup_branch_id);
CREATE INDEX idx_reservations_status ON reservations(status);

CREATE INDEX idx_fines_member ON fines(member_id);
CREATE INDEX idx_fines_loan ON fines(loan_id);
CREATE INDEX idx_fines_status ON fines(status);

CREATE INDEX idx_fine_payments_fine ON fine_payments(fine_id);

CREATE TRIGGER trg_loan_insert_set_copy_status
AFTER INSERT ON loans
WHEN NEW.status = 'active'
BEGIN
    UPDATE book_copies SET status = 'on_loan' WHERE copy_id = NEW.copy_id;
END;

CREATE TRIGGER trg_loan_update_returned_set_copy_status
AFTER UPDATE OF returned_date ON loans
WHEN NEW.returned_date IS NOT NULL AND OLD.returned_date IS NULL
BEGIN
    UPDATE loans SET status = 'returned' WHERE loan_id = NEW.loan_id;
    UPDATE book_copies SET status = 'available' WHERE copy_id = NEW.copy_id;
END;

CREATE TRIGGER trg_fine_payment_insert_update_fine
AFTER INSERT ON fine_payments
BEGIN
    UPDATE fines
    SET amount_paid = amount_paid + NEW.amount,
        status = CASE
            WHEN amount_paid + NEW.amount >= amount THEN 'paid'
            WHEN amount_paid + NEW.amount > 0 THEN 'partially_paid'
            ELSE status
        END,
        paid_date = CASE
            WHEN amount_paid + NEW.amount >= amount THEN date('now')
            ELSE paid_date
        END
    WHERE fine_id = NEW.fine_id;
END;

CREATE VIEW v_overdue_loans AS
SELECT
    l.loan_id,
    m.member_id,
    m.first_name || ' ' || m.last_name AS member_name,
    b.title,
    bc.barcode,
    br.name AS branch_name,
    l.due_date,
    CAST(julianday('now') - julianday(l.due_date) AS INTEGER) AS days_overdue
FROM loans l
JOIN book_copies bc ON bc.copy_id = l.copy_id
JOIN books b ON b.book_id = bc.book_id
JOIN members m ON m.member_id = l.member_id
JOIN branches br ON br.branch_id = l.branch_id
WHERE l.status = 'active'
  AND l.returned_date IS NULL
  AND l.due_date < datetime('now');

CREATE VIEW v_book_availability AS
SELECT
    bk.book_id,
    bk.title,
    bc.branch_id,
    br.name AS branch_name,
    COUNT(*) AS total_copies,
    SUM(CASE WHEN bc.status = 'available' THEN 1 ELSE 0 END) AS available_copies
FROM books bk
JOIN book_copies bc ON bc.book_id = bk.book_id
JOIN branches br ON br.branch_id = bc.branch_id
GROUP BY bk.book_id, bc.branch_id;

CREATE VIEW v_member_outstanding_fines AS
SELECT
    m.member_id,
    m.first_name || ' ' || m.last_name AS member_name,
    SUM(f.amount - f.amount_paid) AS total_outstanding
FROM members m
JOIN fines f ON f.member_id = m.member_id
WHERE f.status IN ('unpaid','partially_paid')
GROUP BY m.member_id;
`
