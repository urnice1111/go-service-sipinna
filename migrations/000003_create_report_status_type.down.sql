BEGIN;

DROP TRIGGER IF EXISTS trg_reportes_first_status ON reportes;

ALTER TABLE historial_estados
  ALTER COLUMN estado TYPE varchar USING estado::text;

DROP TYPE IF EXISTS report_status;

COMMIT;