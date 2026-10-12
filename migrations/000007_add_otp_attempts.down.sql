BEGIN;
    ALTER TABLE otp_verificaciones
    DROP COLUMN intentos;
COMMIT;
