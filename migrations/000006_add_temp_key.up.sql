BEGIN;
    ALTER TABLE reportes
    DROP COLUMN temp_key;
COMMIT;

