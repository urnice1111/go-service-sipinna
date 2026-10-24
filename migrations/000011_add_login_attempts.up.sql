BEGIN;
CREATE TABLE login_attempts (
    contact_hash text PRIMARY KEY,
    attempts integer NOT NULL CHECK (attempts > 0),
    expires_at timestamptz NOT NULL
);
COMMIT;