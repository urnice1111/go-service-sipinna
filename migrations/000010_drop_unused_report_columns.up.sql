-- Columnas de colonia, contacto y eliminación programada que ya no se usan.
BEGIN;

ALTER TABLE reportes
  DROP COLUMN IF EXISTS colonia,
  DROP COLUMN IF EXISTS contacto_email,
  DROP COLUMN IF EXISTS contacto_telefono,
  DROP COLUMN IF EXISTS fecha_eliminacion_programada;

COMMIT;
