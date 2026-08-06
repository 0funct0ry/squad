package elearning

const Schema = `-- E-Learning Platform Database Schema

PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    full_name       TEXT NOT NULL,
    avatar_url      TEXT,
    bio             TEXT,
    role            TEXT NOT NULL DEFAULT 'student'
                        CHECK (role IN ('student', 'instructor', 'admin')),
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_users_role  ON users(role);
CREATE INDEX idx_users_email ON users(email);

CREATE TABLE instructor_profiles (
    instructor_id   INTEGER PRIMARY KEY,
    headline        TEXT,
    website_url     TEXT,
    total_students  INTEGER NOT NULL DEFAULT 0,
    average_rating  REAL NOT NULL DEFAULT 0.0,
    payout_email    TEXT,
    FOREIGN KEY (instructor_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE categories (
    category_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    slug            TEXT NOT NULL UNIQUE,
    parent_id       INTEGER,
    FOREIGN KEY (parent_id) REFERENCES categories(category_id) ON DELETE SET NULL
);

CREATE TABLE courses (
    course_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    instructor_id   INTEGER NOT NULL,
    category_id     INTEGER,
    title           TEXT NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    subtitle        TEXT,
    description     TEXT,
    thumbnail_url   TEXT,
    promo_video_url TEXT,
    level           TEXT NOT NULL DEFAULT 'all_levels'
                        CHECK (level IN ('beginner','intermediate','advanced','all_levels')),
    language        TEXT NOT NULL DEFAULT 'en',
    price           REAL NOT NULL DEFAULT 0.0 CHECK (price >= 0),
    currency        TEXT NOT NULL DEFAULT 'USD',
    status          TEXT NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft','pending_review','published','archived')),
    total_duration_seconds INTEGER NOT NULL DEFAULT 0,
    average_rating  REAL NOT NULL DEFAULT 0.0,
    rating_count    INTEGER NOT NULL DEFAULT 0,
    enrollment_count INTEGER NOT NULL DEFAULT 0,
    published_at    TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (instructor_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(category_id) ON DELETE SET NULL
);

CREATE INDEX idx_courses_instructor ON courses(instructor_id);
CREATE INDEX idx_courses_category   ON courses(category_id);
CREATE INDEX idx_courses_status     ON courses(status);

CREATE TABLE course_co_instructors (
    course_id       INTEGER NOT NULL,
    instructor_id   INTEGER NOT NULL,
    PRIMARY KEY (course_id, instructor_id),
    FOREIGN KEY (course_id) REFERENCES courses(course_id) ON DELETE CASCADE,
    FOREIGN KEY (instructor_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE sections (
    section_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id       INTEGER NOT NULL,
    title           TEXT NOT NULL,
    position        INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (course_id) REFERENCES courses(course_id) ON DELETE CASCADE
);

CREATE INDEX idx_sections_course ON sections(course_id);

CREATE TABLE lessons (
    lesson_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    section_id      INTEGER NOT NULL,
    title           TEXT NOT NULL,
    lesson_type     TEXT NOT NULL DEFAULT 'video'
                        CHECK (lesson_type IN ('video','article','quiz','assignment','resource')),
    content_url     TEXT,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    position        INTEGER NOT NULL DEFAULT 0,
    is_previewable  INTEGER NOT NULL DEFAULT 0 CHECK (is_previewable IN (0,1)),
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (section_id) REFERENCES sections(section_id) ON DELETE CASCADE
);

CREATE INDEX idx_lessons_section ON lessons(section_id);

CREATE TABLE quizzes (
    quiz_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    lesson_id       INTEGER NOT NULL UNIQUE,
    passing_score   INTEGER NOT NULL DEFAULT 70 CHECK (passing_score BETWEEN 0 AND 100),
    max_attempts    INTEGER NOT NULL DEFAULT 3,
    FOREIGN KEY (lesson_id) REFERENCES lessons(lesson_id) ON DELETE CASCADE
);

CREATE TABLE quiz_questions (
    question_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    quiz_id         INTEGER NOT NULL,
    question_text   TEXT NOT NULL,
    question_type   TEXT NOT NULL DEFAULT 'single_choice'
                        CHECK (question_type IN ('single_choice','multiple_choice','true_false')),
    position        INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (quiz_id) REFERENCES quizzes(quiz_id) ON DELETE CASCADE
);

CREATE TABLE quiz_options (
    option_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    question_id     INTEGER NOT NULL,
    option_text     TEXT NOT NULL,
    is_correct      INTEGER NOT NULL DEFAULT 0 CHECK (is_correct IN (0,1)),
    FOREIGN KEY (question_id) REFERENCES quiz_questions(question_id) ON DELETE CASCADE
);

CREATE TABLE quiz_attempts (
    attempt_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    quiz_id         INTEGER NOT NULL,
    student_id      INTEGER NOT NULL,
    score           REAL NOT NULL DEFAULT 0,
    passed          INTEGER NOT NULL DEFAULT 0 CHECK (passed IN (0,1)),
    attempt_number  INTEGER NOT NULL DEFAULT 1,
    started_at      TEXT NOT NULL DEFAULT (datetime('now')),
    submitted_at    TEXT,
    FOREIGN KEY (quiz_id) REFERENCES quizzes(quiz_id) ON DELETE CASCADE,
    FOREIGN KEY (student_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_quiz_attempts_student ON quiz_attempts(student_id);

CREATE TABLE enrollments (
    enrollment_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id      INTEGER NOT NULL,
    course_id       INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active','completed','dropped','refunded')),
    progress_percent REAL NOT NULL DEFAULT 0.0 CHECK (progress_percent BETWEEN 0 AND 100),
    enrolled_at     TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at    TEXT,
    last_accessed_at TEXT,
    UNIQUE (student_id, course_id),
    FOREIGN KEY (student_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (course_id) REFERENCES courses(course_id) ON DELETE CASCADE
);

CREATE INDEX idx_enrollments_student ON enrollments(student_id);
CREATE INDEX idx_enrollments_course  ON enrollments(course_id);
CREATE INDEX idx_enrollments_status  ON enrollments(status);

CREATE TABLE lesson_progress (
    progress_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    enrollment_id   INTEGER NOT NULL,
    lesson_id       INTEGER NOT NULL,
    status          TEXT NOT NULL DEFAULT 'not_started'
                        CHECK (status IN ('not_started','in_progress','completed')),
    watched_seconds INTEGER NOT NULL DEFAULT 0,
    last_position_seconds INTEGER NOT NULL DEFAULT 0,
    completed_at    TEXT,
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (enrollment_id, lesson_id),
    FOREIGN KEY (enrollment_id) REFERENCES enrollments(enrollment_id) ON DELETE CASCADE,
    FOREIGN KEY (lesson_id) REFERENCES lessons(lesson_id) ON DELETE CASCADE
);

CREATE INDEX idx_lesson_progress_enrollment ON lesson_progress(enrollment_id);
CREATE INDEX idx_lesson_progress_lesson     ON lesson_progress(lesson_id);

CREATE TABLE certificates (
    certificate_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    enrollment_id    INTEGER NOT NULL UNIQUE,
    certificate_code TEXT NOT NULL UNIQUE,
    issued_at        TEXT NOT NULL DEFAULT (datetime('now')),
    pdf_url          TEXT,
    FOREIGN KEY (enrollment_id) REFERENCES enrollments(enrollment_id) ON DELETE CASCADE
);

CREATE INDEX idx_certificates_code ON certificates(certificate_code);

CREATE TABLE payments (
    payment_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id      INTEGER NOT NULL,
    course_id       INTEGER NOT NULL,
    enrollment_id   INTEGER,
    amount          REAL NOT NULL CHECK (amount >= 0),
    currency        TEXT NOT NULL DEFAULT 'USD',
    payment_method  TEXT NOT NULL DEFAULT 'card'
                        CHECK (payment_method IN ('card','paypal','wallet','bank_transfer')),
    status          TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','completed','failed','refunded')),
    transaction_ref TEXT UNIQUE,
    paid_at         TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (student_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (course_id) REFERENCES courses(course_id) ON DELETE CASCADE,
    FOREIGN KEY (enrollment_id) REFERENCES enrollments(enrollment_id) ON DELETE SET NULL
);

CREATE INDEX idx_payments_student ON payments(student_id);
CREATE INDEX idx_payments_course  ON payments(course_id);

CREATE TABLE reviews (
    review_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id       INTEGER NOT NULL,
    student_id      INTEGER NOT NULL,
    rating          INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    review_text     TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (course_id, student_id),
    FOREIGN KEY (course_id) REFERENCES courses(course_id) ON DELETE CASCADE,
    FOREIGN KEY (student_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_reviews_course ON reviews(course_id);

CREATE TABLE discussion_threads (
    thread_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    course_id       INTEGER NOT NULL,
    lesson_id       INTEGER,
    student_id      INTEGER NOT NULL,
    title           TEXT NOT NULL,
    body            TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (course_id) REFERENCES courses(course_id) ON DELETE CASCADE,
    FOREIGN KEY (lesson_id) REFERENCES lessons(lesson_id) ON DELETE SET NULL,
    FOREIGN KEY (student_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE discussion_replies (
    reply_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id       INTEGER NOT NULL,
    user_id         INTEGER NOT NULL,
    body            TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (thread_id) REFERENCES discussion_threads(thread_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_threads_course ON discussion_threads(course_id);
CREATE INDEX idx_replies_thread ON discussion_replies(thread_id);

CREATE TABLE coupons (
    coupon_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    code            TEXT NOT NULL UNIQUE,
    discount_type   TEXT NOT NULL DEFAULT 'percent'
                        CHECK (discount_type IN ('percent','fixed')),
    discount_value  REAL NOT NULL CHECK (discount_value > 0),
    course_id       INTEGER,
    max_uses        INTEGER,
    used_count      INTEGER NOT NULL DEFAULT 0,
    valid_from      TEXT,
    valid_until     TEXT,
    FOREIGN KEY (course_id) REFERENCES courses(course_id) ON DELETE CASCADE
);

CREATE TRIGGER trg_users_updated_at
AFTER UPDATE ON users
BEGIN
    UPDATE users SET updated_at = datetime('now') WHERE user_id = NEW.user_id;
END;

CREATE TRIGGER trg_courses_updated_at
AFTER UPDATE ON courses
BEGIN
    UPDATE courses SET updated_at = datetime('now') WHERE course_id = NEW.course_id;
END;

CREATE TRIGGER trg_enrollment_count_insert
AFTER INSERT ON enrollments
BEGIN
    UPDATE courses SET enrollment_count = enrollment_count + 1
    WHERE course_id = NEW.course_id;
END;

CREATE TRIGGER trg_review_insert_update_rating
AFTER INSERT ON reviews
BEGIN
    UPDATE courses
    SET rating_count = rating_count + 1,
        average_rating = (
            SELECT AVG(rating) FROM reviews WHERE course_id = NEW.course_id
        )
    WHERE course_id = NEW.course_id;
END;
`
