BEGIN;

ALTER TABLE historial_estados
  ALTER COLUMN estado TYPE varchar USING estado::text;

DROP TYPE IF EXISTS report_status;

COMMIT;