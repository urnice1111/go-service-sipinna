BEGIN;

ALTER TABLE reportes
ADD COLUMN temp_key uuid;

COMMIT;
