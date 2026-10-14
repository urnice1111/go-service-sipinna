BEGIN;

DROP INDEX idx_casos_zona;

ALTER TABLE casos
DROP COLUMN zona_id;

COMMIT;
