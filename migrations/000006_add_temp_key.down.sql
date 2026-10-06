BEGIN;

ALTER TABLE reportes
ADD temp_key uuid;

COMMIT;
