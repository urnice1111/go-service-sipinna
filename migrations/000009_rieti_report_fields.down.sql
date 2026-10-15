-- Los valores agregados a report_status no se pueden quitar de un enum en Postgres;
-- se quedan aunque ya no se usen.
BEGIN;

DROP TABLE IF EXISTS bitacora;

ALTER TABLE reportes
  DROP COLUMN IF EXISTS riesgo,
  DROP COLUMN IF EXISTS peligro_inmediato,
  DROP COLUMN IF EXISTS colonia,
  DROP COLUMN IF EXISTS referencia,
  DROP COLUMN IF EXISTS contacto_email,
  DROP COLUMN IF EXISTS contacto_telefono,
  DROP COLUMN IF EXISTS clave_consulta,
  DROP COLUMN IF EXISTS prioridad,
  DROP COLUMN IF EXISTS sexo,
  DROP COLUMN IF EXISTS posible_duplicado_de,
  DROP COLUMN IF EXISTS responsable,
  DROP COLUMN IF EXISTS dependencia_canalizada,
  DROP COLUMN IF EXISTS fecha_seguimiento,
  DROP COLUMN IF EXISTS resultado,
  DROP COLUMN IF EXISTS nna_atendidos,
  DROP COLUMN IF EXISTS fecha_cierre;

COMMIT;
