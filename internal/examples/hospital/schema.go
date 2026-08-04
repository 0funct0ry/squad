package hospital

const Schema = `-- Hospital/EHR System Schema
PRAGMA foreign_keys = ON;

CREATE TABLE departments (
    department_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL UNIQUE,
    location           TEXT,
    phone              TEXT,
    created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE specialties (
    specialty_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL UNIQUE
);

CREATE TABLE staff (
    staff_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name         TEXT NOT NULL,
    last_name          TEXT NOT NULL,
    role               TEXT NOT NULL CHECK (role IN ('doctor','nurse','admin','technician','pharmacist')),
    email              TEXT UNIQUE,
    phone              TEXT,
    department_id      INTEGER REFERENCES departments(department_id) ON DELETE SET NULL,
    hire_date          TEXT,
    is_active          INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_staff_department ON staff(department_id);
CREATE INDEX idx_staff_role ON staff(role);

CREATE TABLE doctors (
    doctor_id          INTEGER PRIMARY KEY REFERENCES staff(staff_id) ON DELETE CASCADE,
    license_number     TEXT NOT NULL UNIQUE,
    years_experience    INTEGER CHECK (years_experience >= 0),
    consultation_fee   REAL CHECK (consultation_fee >= 0)
);

CREATE TABLE doctor_specialties (
    doctor_id          INTEGER NOT NULL REFERENCES doctors(doctor_id) ON DELETE CASCADE,
    specialty_id       INTEGER NOT NULL REFERENCES specialties(specialty_id) ON DELETE CASCADE,
    PRIMARY KEY (doctor_id, specialty_id)
);

CREATE TABLE patients (
    patient_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name         TEXT NOT NULL,
    last_name          TEXT NOT NULL,
    date_of_birth      TEXT NOT NULL,
    gender             TEXT CHECK (gender IN ('male','female','other','unknown')),
    blood_type         TEXT CHECK (blood_type IN ('A+','A-','B+','B-','AB+','AB-','O+','O-','unknown')),
    email              TEXT UNIQUE,
    phone              TEXT,
    address_line1      TEXT,
    address_line2      TEXT,
    city               TEXT,
    state              TEXT,
    postal_code        TEXT,
    country            TEXT,
    emergency_contact_name  TEXT,
    emergency_contact_phone TEXT,
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_patients_name ON patients(last_name, first_name);
CREATE INDEX idx_patients_dob ON patients(date_of_birth);

CREATE TABLE patient_allergies (
    allergy_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id         INTEGER NOT NULL REFERENCES patients(patient_id) ON DELETE CASCADE,
    allergen           TEXT NOT NULL,
    reaction           TEXT,
    severity           TEXT CHECK (severity IN ('mild','moderate','severe','unknown')),
    recorded_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_allergies_patient ON patient_allergies(patient_id);

CREATE TABLE insurance_providers (
    provider_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL UNIQUE,
    contact_phone      TEXT,
    contact_email      TEXT
);

CREATE TABLE patient_insurance (
    patient_insurance_id INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id         INTEGER NOT NULL REFERENCES patients(patient_id) ON DELETE CASCADE,
    provider_id        INTEGER NOT NULL REFERENCES insurance_providers(provider_id) ON DELETE RESTRICT,
    policy_number      TEXT NOT NULL,
    group_number       TEXT,
    valid_from         TEXT,
    valid_to           TEXT,
    is_primary         INTEGER NOT NULL DEFAULT 1 CHECK (is_primary IN (0,1)),
    UNIQUE (patient_id, policy_number)
);

CREATE INDEX idx_patient_insurance_patient ON patient_insurance(patient_id);

CREATE TABLE appointments (
    appointment_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id         INTEGER NOT NULL REFERENCES patients(patient_id) ON DELETE CASCADE,
    doctor_id          INTEGER NOT NULL REFERENCES doctors(doctor_id) ON DELETE RESTRICT,
    department_id      INTEGER REFERENCES departments(department_id) ON DELETE SET NULL,
    scheduled_start    TEXT NOT NULL,
    scheduled_end      TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'scheduled'
                         CHECK (status IN ('scheduled','checked_in','in_progress','completed','cancelled','no_show')),
    reason             TEXT,
    notes              TEXT,
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    CHECK (scheduled_end > scheduled_start)
);

CREATE INDEX idx_appointments_patient ON appointments(patient_id);
CREATE INDEX idx_appointments_doctor ON appointments(doctor_id);
CREATE INDEX idx_appointments_start ON appointments(scheduled_start);
CREATE INDEX idx_appointments_status ON appointments(status);

CREATE TABLE medical_records (
    record_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id         INTEGER NOT NULL REFERENCES patients(patient_id) ON DELETE CASCADE,
    doctor_id          INTEGER NOT NULL REFERENCES doctors(doctor_id) ON DELETE RESTRICT,
    appointment_id     INTEGER REFERENCES appointments(appointment_id) ON DELETE SET NULL,
    visit_date         TEXT NOT NULL DEFAULT (datetime('now')),
    chief_complaint    TEXT,
    diagnosis          TEXT,
    icd10_code         TEXT,
    treatment_plan     TEXT,
    notes              TEXT,
    created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_records_patient ON medical_records(patient_id);
CREATE INDEX idx_records_doctor ON medical_records(doctor_id);
CREATE INDEX idx_records_appointment ON medical_records(appointment_id);
CREATE INDEX idx_records_icd10 ON medical_records(icd10_code);

CREATE TABLE vital_signs (
    vital_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    record_id          INTEGER NOT NULL REFERENCES medical_records(record_id) ON DELETE CASCADE,
    recorded_at        TEXT NOT NULL DEFAULT (datetime('now')),
    height_cm          REAL,
    weight_kg          REAL,
    temperature_c      REAL,
    blood_pressure_systolic  INTEGER,
    blood_pressure_diastolic INTEGER,
    heart_rate_bpm     INTEGER,
    respiratory_rate   INTEGER,
    oxygen_saturation  REAL
);

CREATE INDEX idx_vitals_record ON vital_signs(record_id);

CREATE TABLE medications (
    medication_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL,
    generic_name       TEXT,
    form                TEXT CHECK (form IN ('tablet','capsule','liquid','injection','topical','inhaler','other')),
    strength           TEXT,
    manufacturer       TEXT,
    UNIQUE (name, strength, form)
);

CREATE TABLE prescriptions (
    prescription_id    INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id         INTEGER NOT NULL REFERENCES patients(patient_id) ON DELETE CASCADE,
    doctor_id          INTEGER NOT NULL REFERENCES doctors(doctor_id) ON DELETE RESTRICT,
    record_id          INTEGER REFERENCES medical_records(record_id) ON DELETE SET NULL,
    prescribed_date    TEXT NOT NULL DEFAULT (datetime('now')),
    status             TEXT NOT NULL DEFAULT 'active'
                         CHECK (status IN ('active','completed','cancelled','expired')),
    notes              TEXT
);

CREATE INDEX idx_prescriptions_patient ON prescriptions(patient_id);
CREATE INDEX idx_prescriptions_doctor ON prescriptions(doctor_id);

CREATE TABLE prescription_items (
    prescription_item_id INTEGER PRIMARY KEY AUTOINCREMENT,
    prescription_id    INTEGER NOT NULL REFERENCES prescriptions(prescription_id) ON DELETE CASCADE,
    medication_id      INTEGER NOT NULL REFERENCES medications(medication_id) ON DELETE RESTRICT,
    dosage             TEXT NOT NULL,
    frequency          TEXT NOT NULL,
    duration_days      INTEGER CHECK (duration_days > 0),
    quantity           INTEGER CHECK (quantity > 0),
    instructions       TEXT
);

CREATE INDEX idx_rx_items_prescription ON prescription_items(prescription_id);
CREATE INDEX idx_rx_items_medication ON prescription_items(medication_id);

CREATE TABLE lab_tests (
    lab_test_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL UNIQUE,
    unit               TEXT,
    reference_range    TEXT
);

CREATE TABLE lab_orders (
    lab_order_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id         INTEGER NOT NULL REFERENCES patients(patient_id) ON DELETE CASCADE,
    doctor_id          INTEGER NOT NULL REFERENCES doctors(doctor_id) ON DELETE RESTRICT,
    record_id          INTEGER REFERENCES medical_records(record_id) ON DELETE SET NULL,
    ordered_at         TEXT NOT NULL DEFAULT (datetime('now')),
    status             TEXT NOT NULL DEFAULT 'ordered'
                         CHECK (status IN ('ordered','collected','in_progress','completed','cancelled'))
);

CREATE INDEX idx_lab_orders_patient ON lab_orders(patient_id);

CREATE TABLE lab_results (
    lab_result_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    lab_order_id       INTEGER NOT NULL REFERENCES lab_orders(lab_order_id) ON DELETE CASCADE,
    lab_test_id        INTEGER NOT NULL REFERENCES lab_tests(lab_test_id) ON DELETE RESTRICT,
    result_value       TEXT,
    is_abnormal        INTEGER DEFAULT 0 CHECK (is_abnormal IN (0,1)),
    resulted_at        TEXT DEFAULT (datetime('now')),
    notes              TEXT
);

CREATE INDEX idx_lab_results_order ON lab_results(lab_order_id);

CREATE TABLE billing_items (
    billing_item_id    INTEGER PRIMARY KEY AUTOINCREMENT,
    code               TEXT NOT NULL UNIQUE,
    description        TEXT NOT NULL,
    unit_price         REAL NOT NULL CHECK (unit_price >= 0)
);

CREATE TABLE invoices (
    invoice_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    patient_id         INTEGER NOT NULL REFERENCES patients(patient_id) ON DELETE CASCADE,
    appointment_id     INTEGER REFERENCES appointments(appointment_id) ON DELETE SET NULL,
    invoice_date       TEXT NOT NULL DEFAULT (datetime('now')),
    due_date           TEXT,
    status             TEXT NOT NULL DEFAULT 'unpaid'
                         CHECK (status IN ('unpaid','partially_paid','paid','void','overdue')),
    total_amount       REAL NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    amount_paid        REAL NOT NULL DEFAULT 0 CHECK (amount_paid >= 0),
    created_at         TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_invoices_patient ON invoices(patient_id);
CREATE INDEX idx_invoices_status ON invoices(status);

CREATE TABLE invoice_line_items (
    line_item_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id         INTEGER NOT NULL REFERENCES invoices(invoice_id) ON DELETE CASCADE,
    billing_item_id    INTEGER NOT NULL REFERENCES billing_items(billing_item_id) ON DELETE RESTRICT,
    quantity           INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price         REAL NOT NULL CHECK (unit_price >= 0),
    line_total         REAL GENERATED ALWAYS AS (quantity * unit_price) VIRTUAL
);

CREATE INDEX idx_invoice_lines_invoice ON invoice_line_items(invoice_id);

CREATE TABLE payments (
    payment_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id         INTEGER NOT NULL REFERENCES invoices(invoice_id) ON DELETE CASCADE,
    payment_date       TEXT NOT NULL DEFAULT (datetime('now')),
    amount             REAL NOT NULL CHECK (amount > 0),
    method             TEXT CHECK (method IN ('cash','credit_card','debit_card','insurance','bank_transfer','other')),
    reference_number   TEXT
);

CREATE INDEX idx_payments_invoice ON payments(invoice_id);

CREATE TABLE insurance_claims (
    claim_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id         INTEGER NOT NULL REFERENCES invoices(invoice_id) ON DELETE CASCADE,
    patient_insurance_id INTEGER NOT NULL REFERENCES patient_insurance(patient_insurance_id) ON DELETE RESTRICT,
    claim_date         TEXT NOT NULL DEFAULT (datetime('now')),
    claim_amount       REAL NOT NULL CHECK (claim_amount >= 0),
    approved_amount    REAL CHECK (approved_amount >= 0),
    status             TEXT NOT NULL DEFAULT 'submitted'
                         CHECK (status IN ('submitted','under_review','approved','denied','paid'))
);

CREATE INDEX idx_claims_invoice ON insurance_claims(invoice_id);


CREATE TRIGGER trg_patients_updated_at
AFTER UPDATE ON patients
FOR EACH ROW
BEGIN
    UPDATE patients SET updated_at = datetime('now') WHERE patient_id = OLD.patient_id;
END;

CREATE TRIGGER trg_invoice_total_after_insert
AFTER INSERT ON invoice_line_items
FOR EACH ROW
BEGIN
    UPDATE invoices
    SET total_amount = (SELECT COALESCE(SUM(quantity * unit_price), 0)
                         FROM invoice_line_items WHERE invoice_id = NEW.invoice_id)
    WHERE invoice_id = NEW.invoice_id;
END;

CREATE TRIGGER trg_invoice_total_after_update
AFTER UPDATE ON invoice_line_items
FOR EACH ROW
BEGIN
    UPDATE invoices
    SET total_amount = (SELECT COALESCE(SUM(quantity * unit_price), 0)
                         FROM invoice_line_items WHERE invoice_id = NEW.invoice_id)
    WHERE invoice_id = NEW.invoice_id;
END;

CREATE TRIGGER trg_invoice_total_after_delete
AFTER DELETE ON invoice_line_items
FOR EACH ROW
BEGIN
    UPDATE invoices
    SET total_amount = (SELECT COALESCE(SUM(quantity * unit_price), 0)
                         FROM invoice_line_items WHERE invoice_id = OLD.invoice_id)
    WHERE invoice_id = OLD.invoice_id;
END;

CREATE TRIGGER trg_payment_updates_invoice
AFTER INSERT ON payments
FOR EACH ROW
BEGIN
    UPDATE invoices
    SET amount_paid = (SELECT COALESCE(SUM(amount), 0) FROM payments WHERE invoice_id = NEW.invoice_id),
        status = CASE
            WHEN (SELECT COALESCE(SUM(amount), 0) FROM payments WHERE invoice_id = NEW.invoice_id) >= total_amount
                THEN 'paid'
            WHEN (SELECT COALESCE(SUM(amount), 0) FROM payments WHERE invoice_id = NEW.invoice_id) > 0
                THEN 'partially_paid'
            ELSE status
        END
    WHERE invoice_id = NEW.invoice_id;
END;

CREATE VIEW v_upcoming_appointments AS
SELECT a.appointment_id, p.first_name || ' ' || p.last_name AS patient_name,
       s.first_name || ' ' || s.last_name AS doctor_name,
       a.scheduled_start, a.scheduled_end, a.status, d.name AS department
FROM appointments a
JOIN patients p ON p.patient_id = a.patient_id
JOIN doctors doc ON doc.doctor_id = a.doctor_id
JOIN staff s ON s.staff_id = doc.doctor_id
LEFT JOIN departments d ON d.department_id = a.department_id
WHERE a.status IN ('scheduled','checked_in')
ORDER BY a.scheduled_start;

CREATE VIEW v_patient_balance AS
SELECT p.patient_id, p.first_name || ' ' || p.last_name AS patient_name,
       COALESCE(SUM(i.total_amount), 0) AS total_billed,
       COALESCE(SUM(i.amount_paid), 0) AS total_paid,
       COALESCE(SUM(i.total_amount - i.amount_paid), 0) AS balance_due
FROM patients p
LEFT JOIN invoices i ON i.patient_id = p.patient_id
GROUP BY p.patient_id;

CREATE VIEW v_active_prescriptions AS
SELECT rx.prescription_id, p.first_name || ' ' || p.last_name AS patient_name,
       m.name AS medication_name, pi.dosage, pi.frequency, pi.duration_days,
       rx.prescribed_date, rx.status
FROM prescriptions rx
JOIN patients p ON p.patient_id = rx.patient_id
JOIN prescription_items pi ON pi.prescription_id = rx.prescription_id
JOIN medications m ON m.medication_id = pi.medication_id
WHERE rx.status = 'active';
`
