BEGIN;

-- Intentos fallidos de un código; al llegar al límite el código se bloquea.
ALTER TABLE otp_verificaciones
ADD intentos int NOT NULL DEFAULT 0;

COMMIT;
