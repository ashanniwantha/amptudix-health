-- migrations/00001_create_users.sql
-- +goose Up

-- gen_random_uuid() is native from PostgreSQL 13; the extension is only
-- needed on older versions. uuidv7() is native from PostgreSQL 18 and is
-- preferred when available because it is time-ordered.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ow_id           UUID UNIQUE,                    -- NULL until linked to Open Wearables
    username        TEXT NOT NULL,
    email           TEXT NOT NULL,
    hashed_password TEXT NOT NULL,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    dob             DATE,                           -- NULL until supplied
    deleted_at      TIMESTAMPTZ,                    -- NULL = active; soft delete
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Identity constraints
    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT uq_users_ow_id UNIQUE (ow_id),

    -- Username: 3–32 chars, letters/digits/underscore (mirrors domain.go)
    CONSTRAINT chk_users_username_length
        CHECK (length(trim(username)) BETWEEN 3 AND 32),
    CONSTRAINT chk_users_username_chars
        CHECK (username ~ '^[A-Za-z0-9_]+$'),

    -- Email: non-empty and basic format
    CONSTRAINT chk_users_email_not_empty
        CHECK (length(trim(email)) > 0),
    CONSTRAINT chk_users_email_format
        CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),

    -- Names: non-empty (mirrors ValidateName)
    CONSTRAINT chk_users_first_name_not_empty
        CHECK (length(trim(first_name)) > 0),
    CONSTRAINT chk_users_last_name_not_empty
        CHECK (length(trim(last_name)) > 0),

    -- Password hash must not be empty
    CONSTRAINT chk_users_hashed_password_not_empty
        CHECK (length(hashed_password) > 0),

    -- DOB: must not be in the future, and must be at least 13 years old.
    -- We DO NOT enforce the 120-year upper bound here: as current_date
    -- advances, a row valid today could become invalid tomorrow, violating
    -- the "monotonically weaker" rule for CHECK constraints. The upper bound
    -- is enforced in the application (ValidateDOB) where it can use the
    -- current clock without breaking the schema.
    CONSTRAINT chk_users_dob_not_future
        CHECK (dob IS NULL OR dob <= CURRENT_DATE),
    CONSTRAINT chk_users_dob_min_age
        CHECK (dob IS NULL OR dob <= CURRENT_DATE - INTERVAL '13 years')
);

-- Case-insensitive uniqueness for email. This replaces citext and is
-- significantly faster. The COLLATE "C" hint keeps comparisons in the
-- fast C locale.
CREATE UNIQUE INDEX uq_users_email_lower
    ON users (LOWER(email) COLLATE "C")
    WHERE deleted_at IS NULL;

-- Case-insensitive uniqueness for username, matching the same
-- "case-preserving but unique" convention. If you want usernames to be
-- case-sensitive, drop this and use a plain UNIQUE (username) instead.
CREATE UNIQUE INDEX uq_users_username_lower
    ON users (LOWER(username) COLLATE "C")
    WHERE deleted_at IS NULL;

-- Trigger function: reusable across every table that has updated_at.
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_users_set_updated_at ON users;
DROP FUNCTION IF EXISTS set_updated_at();
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS pgcrypto;
