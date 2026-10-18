-- Restaura las columnas vacías; los datos borrados no se recuperan.
BEGIN;

ALTER TABLE reportes
  ADD COLUMN IF NOT EXISTS colonia                      varchar,
  ADD COLUMN IF NOT EXISTS contacto_email               varchar,
  ADD COLUMN IF NOT EXISTS contacto_telefono            varchar,
  ADD COLUMN IF NOT EXISTS fecha_eliminacion_programada date;

COMMIT;
