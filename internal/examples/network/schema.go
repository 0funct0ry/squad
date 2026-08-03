package network

const Schema = `
-- Minimal professional networking schema

PRAGMA foreign_keys = ON;

CREATE TABLE users (
    user_id             INTEGER PRIMARY KEY AUTOINCREMENT,
    email               TEXT NOT NULL UNIQUE,
    phone               TEXT,
    password_hash       TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active','suspended','deactivated','deleted')),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    last_login_at       TEXT
);

CREATE TABLE profiles (
    profile_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id             INTEGER NOT NULL UNIQUE,
    first_name          TEXT NOT NULL,
    last_name            TEXT NOT NULL,
    headline            TEXT,                       -- e.g. "Senior Data Engineer at Acme"
    summary             TEXT,
    profile_picture_url TEXT,
    background_image_url TEXT,
    location_city       TEXT,
    location_region     TEXT,
    location_country    TEXT,
    industry            TEXT,
    current_company_id  INTEGER,                     -- FK to companies, nullable
    profile_url_slug    TEXT UNIQUE,                  -- e.g. linkedin.com/in/<slug>
    is_open_to_work      INTEGER NOT NULL DEFAULT 0,   -- boolean 0/1
    is_hiring           INTEGER NOT NULL DEFAULT 0,
    connections_count    INTEGER NOT NULL DEFAULT 0,   -- denormalized counter
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (current_company_id) REFERENCES companies(company_id) ON DELETE SET NULL
);

CREATE TABLE experiences (
    experience_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id          INTEGER NOT NULL,
    company_id          INTEGER,                      -- nullable if company not in DB
    company_name_raw    TEXT,                         -- free-text fallback
    title               TEXT NOT NULL,
    employment_type     TEXT CHECK (employment_type IN
                            ('full_time','part_time','contract','internship','freelance','self_employed')),
    location            TEXT,
    description         TEXT,
    start_date          TEXT NOT NULL,                 -- YYYY-MM or YYYY-MM-DD
    end_date            TEXT,                          -- NULL = current
    is_current          INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    FOREIGN KEY (company_id) REFERENCES companies(company_id) ON DELETE SET NULL
);

CREATE TABLE education (
    education_id        INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id          INTEGER NOT NULL,
    school_name         TEXT NOT NULL,
    degree              TEXT,
    field_of_study      TEXT,
    start_year          INTEGER,
    end_year            INTEGER,
    description         TEXT,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE
);

CREATE TABLE certifications (
    certification_id    INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id          INTEGER NOT NULL,
    name                TEXT NOT NULL,
    issuing_organization TEXT,
    issue_date          TEXT,
    expiration_date     TEXT,
    credential_id        TEXT,
    credential_url       TEXT,
    FOREIGN KEY (profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE
);


CREATE TABLE skills (
    skill_id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT NOT NULL UNIQUE
);

CREATE TABLE profile_skills (
    profile_skill_id    INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_id          INTEGER NOT NULL,
    skill_id            INTEGER NOT NULL,
    display_order       INTEGER NOT NULL DEFAULT 0,
    endorsement_count   INTEGER NOT NULL DEFAULT 0,    -- denormalized counter
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (profile_id, skill_id),
    FOREIGN KEY (profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id) REFERENCES skills(skill_id) ON DELETE CASCADE
);

CREATE TABLE endorsements (
    endorsement_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    profile_skill_id    INTEGER NOT NULL,              -- which profile+skill is endorsed
    endorser_profile_id INTEGER NOT NULL,               -- who gave the endorsement
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (profile_skill_id, endorser_profile_id),     -- one endorsement per person per skill
    FOREIGN KEY (profile_skill_id) REFERENCES profile_skills(profile_skill_id) ON DELETE CASCADE,
    FOREIGN KEY (endorser_profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    CHECK (endorser_profile_id IS NOT NULL)
);

CREATE TABLE connections (
    connection_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    requester_profile_id INTEGER NOT NULL,
    addressee_profile_id INTEGER NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','accepted','declined','blocked','withdrawn')),
    message              TEXT,                          -- optional note sent with request
    requested_at        TEXT NOT NULL DEFAULT (datetime('now')),
    responded_at        TEXT,
    FOREIGN KEY (requester_profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    FOREIGN KEY (addressee_profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    CHECK (requester_profile_id <> addressee_profile_id),
    UNIQUE (requester_profile_id, addressee_profile_id)
);

CREATE TABLE follows (
    follow_id           INTEGER PRIMARY KEY AUTOINCREMENT,
    follower_profile_id INTEGER NOT NULL,
    followee_profile_id INTEGER,                       -- nullable if following a company instead
    followee_company_id INTEGER,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (follower_profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    FOREIGN KEY (followee_profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE,
    FOREIGN KEY (followee_company_id) REFERENCES companies(company_id) ON DELETE CASCADE,
    CHECK (
        (followee_profile_id IS NOT NULL AND followee_company_id IS NULL)
        OR (followee_profile_id IS NULL AND followee_company_id IS NOT NULL)
    ),
    UNIQUE (follower_profile_id, followee_profile_id, followee_company_id)
);


CREATE TABLE companies (
    company_id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT NOT NULL,
    legal_name          TEXT,
    description         TEXT,
    industry            TEXT,
    company_size        TEXT CHECK (company_size IN
                            ('1-10','11-50','51-200','201-500','501-1000','1001-5000','5001-10000','10001+')),
    company_type        TEXT CHECK (company_type IN
                            ('public','private','nonprofit','government','partnership','self_employed')),
    founded_year        INTEGER,
    website_url         TEXT,
    logo_url            TEXT,
    headquarters_city   TEXT,
    headquarters_region TEXT,
    headquarters_country TEXT,
    follower_count       INTEGER NOT NULL DEFAULT 0,    -- denormalized counter
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE company_admins (
    company_admin_id    INTEGER PRIMARY KEY AUTOINCREMENT,
    company_id          INTEGER NOT NULL,
    profile_id          INTEGER NOT NULL,
    role                TEXT NOT NULL DEFAULT 'admin' CHECK (role IN ('admin','super_admin','recruiter_poster')),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (company_id, profile_id),
    FOREIGN KEY (company_id) REFERENCES companies(company_id) ON DELETE CASCADE,
    FOREIGN KEY (profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE
);


CREATE TABLE job_postings (
    job_posting_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    company_id          INTEGER NOT NULL,
    posted_by_profile_id INTEGER,                       -- who created the posting
    title               TEXT NOT NULL,
    description         TEXT NOT NULL,
    employment_type     TEXT CHECK (employment_type IN
                            ('full_time','part_time','contract','internship','temporary','volunteer')),
    experience_level    TEXT CHECK (experience_level IN
                            ('internship','entry_level','associate','mid_senior','director','executive')),
    location_city       TEXT,
    location_region     TEXT,
    location_country    TEXT,
    is_remote           INTEGER NOT NULL DEFAULT 0,
    salary_min          REAL,
    salary_max          REAL,
    salary_currency     TEXT DEFAULT 'USD',
    status              TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','closed','draft','expired')),
    applicant_count     INTEGER NOT NULL DEFAULT 0,      -- denormalized counter
    posted_at           TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at          TEXT,
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (company_id) REFERENCES companies(company_id) ON DELETE CASCADE,
    FOREIGN KEY (posted_by_profile_id) REFERENCES profiles(profile_id) ON DELETE SET NULL
);

CREATE TABLE job_posting_skills (
    job_posting_skill_id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_posting_id      INTEGER NOT NULL,
    skill_id            INTEGER NOT NULL,
    is_required         INTEGER NOT NULL DEFAULT 1,
    UNIQUE (job_posting_id, skill_id),
    FOREIGN KEY (job_posting_id) REFERENCES job_postings(job_posting_id) ON DELETE CASCADE,
    FOREIGN KEY (skill_id) REFERENCES skills(skill_id) ON DELETE CASCADE
);

CREATE TABLE job_applications (
    application_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    job_posting_id      INTEGER NOT NULL,
    applicant_profile_id INTEGER NOT NULL,
    resume_url           TEXT,
    cover_letter         TEXT,
    status              TEXT NOT NULL DEFAULT 'submitted'
                            CHECK (status IN ('submitted','under_review','interviewing','offered','rejected','withdrawn','hired')),
    applied_at           TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (job_posting_id, applicant_profile_id),
    FOREIGN KEY (job_posting_id) REFERENCES job_postings(job_posting_id) ON DELETE CASCADE,
    FOREIGN KEY (applicant_profile_id) REFERENCES profiles(profile_id) ON DELETE CASCADE
);

-- INDEXES

CREATE INDEX idx_profiles_user_id ON profiles(user_id);
CREATE INDEX idx_profiles_current_company ON profiles(current_company_id);
CREATE INDEX idx_experiences_profile ON experiences(profile_id);
CREATE INDEX idx_experiences_company ON experiences(company_id);
CREATE INDEX idx_education_profile ON education(profile_id);
CREATE INDEX idx_certifications_profile ON certifications(profile_id);

CREATE INDEX idx_profile_skills_profile ON profile_skills(profile_id);
CREATE INDEX idx_profile_skills_skill ON profile_skills(skill_id);
CREATE INDEX idx_endorsements_profile_skill ON endorsements(profile_skill_id);
CREATE INDEX idx_endorsements_endorser ON endorsements(endorser_profile_id);

CREATE INDEX idx_connections_requester ON connections(requester_profile_id);
CREATE INDEX idx_connections_addressee ON connections(addressee_profile_id);
CREATE INDEX idx_connections_status ON connections(status);
CREATE INDEX idx_follows_follower ON follows(follower_profile_id);
CREATE INDEX idx_follows_followee_profile ON follows(followee_profile_id);
CREATE INDEX idx_follows_followee_company ON follows(followee_company_id);

CREATE INDEX idx_company_admins_company ON company_admins(company_id);
CREATE INDEX idx_company_admins_profile ON company_admins(profile_id);

CREATE INDEX idx_job_postings_company ON job_postings(company_id);
CREATE INDEX idx_job_postings_status ON job_postings(status);
CREATE INDEX idx_job_posting_skills_posting ON job_posting_skills(job_posting_id);
CREATE INDEX idx_job_posting_skills_skill ON job_posting_skills(skill_id);
CREATE INDEX idx_job_applications_posting ON job_applications(job_posting_id);
CREATE INDEX idx_job_applications_applicant ON job_applications(applicant_profile_id);
CREATE INDEX idx_job_applications_status ON job_applications(status);

-- TRIGGERS

CREATE TRIGGER trg_connection_accepted_inc
AFTER UPDATE OF status ON connections
WHEN NEW.status = 'accepted' AND OLD.status <> 'accepted'
BEGIN
    UPDATE profiles SET connections_count = connections_count + 1 WHERE profile_id = NEW.requester_profile_id;
    UPDATE profiles SET connections_count = connections_count + 1 WHERE profile_id = NEW.addressee_profile_id;
END;

CREATE TRIGGER trg_connection_unaccepted_dec
AFTER UPDATE OF status ON connections
WHEN OLD.status = 'accepted' AND NEW.status <> 'accepted'
BEGIN
    UPDATE profiles SET connections_count = connections_count - 1 WHERE profile_id = NEW.requester_profile_id;
    UPDATE profiles SET connections_count = connections_count - 1 WHERE profile_id = NEW.addressee_profile_id;
END;

CREATE TRIGGER trg_endorsement_insert
AFTER INSERT ON endorsements
BEGIN
    UPDATE profile_skills SET endorsement_count = endorsement_count + 1
    WHERE profile_skill_id = NEW.profile_skill_id;
END;

CREATE TRIGGER trg_endorsement_delete
AFTER DELETE ON endorsements
BEGIN
    UPDATE profile_skills SET endorsement_count = endorsement_count - 1
    WHERE profile_skill_id = OLD.profile_skill_id;
END;

CREATE TRIGGER trg_follow_company_insert
AFTER INSERT ON follows
WHEN NEW.followee_company_id IS NOT NULL
BEGIN
    UPDATE companies SET follower_count = follower_count + 1 WHERE company_id = NEW.followee_company_id;
END;

CREATE TRIGGER trg_follow_company_delete
AFTER DELETE ON follows
WHEN OLD.followee_company_id IS NOT NULL
BEGIN
    UPDATE companies SET follower_count = follower_count - 1 WHERE company_id = OLD.followee_company_id;
END;

CREATE TRIGGER trg_application_insert
AFTER INSERT ON job_applications
BEGIN
    UPDATE job_postings SET applicant_count = applicant_count + 1 WHERE job_posting_id = NEW.job_posting_id;
END;

CREATE TRIGGER trg_application_delete
AFTER DELETE ON job_applications
BEGIN
    UPDATE job_postings SET applicant_count = applicant_count - 1 WHERE job_posting_id = OLD.job_posting_id;
END;

CREATE TRIGGER trg_profiles_updated_at
AFTER UPDATE ON profiles
BEGIN
    UPDATE profiles SET updated_at = datetime('now') WHERE profile_id = NEW.profile_id;
END;

CREATE TRIGGER trg_companies_updated_at
AFTER UPDATE ON companies
BEGIN
    UPDATE companies SET updated_at = datetime('now') WHERE company_id = NEW.company_id;
END;

CREATE TRIGGER trg_job_postings_updated_at
AFTER UPDATE ON job_postings
BEGIN
    UPDATE job_postings SET updated_at = datetime('now') WHERE job_posting_id = NEW.job_posting_id;
END;
`
