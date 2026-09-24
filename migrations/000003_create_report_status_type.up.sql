CREATE TYPE report_status AS ENUM (
  'registrado',
  'en_revision',
  'en_seguimiento',
  'canalizado',
  'concluido',
  'archivado',
  'cancelado',
  'reincidente'
);

-- Valores previos al enum: 'pendiente' (default de reportes) y 'Reporte recibido' (trigger).
-- Cualquier otro valor que no exista en el enum hace fallar la migración.
ALTER TABLE "reportes" ALTER COLUMN "estado" DROP DEFAULT;
ALTER TABLE "reportes" ALTER COLUMN "estado" TYPE report_status USING (
  CASE
    WHEN "estado" IS NULL OR "estado" IN ('pendiente', 'Reporte recibido') THEN 'registrado'
    ELSE "estado"
  END
)::report_status;
ALTER TABLE "reportes" ALTER COLUMN "estado" SET DEFAULT 'registrado';
ALTER TABLE "reportes" ALTER COLUMN "estado" SET NOT NULL;

ALTER TABLE "historial_estados" ALTER COLUMN "estado" TYPE report_status USING (
  CASE
    WHEN "estado" IS NULL OR "estado" IN ('pendiente', 'Reporte recibido') THEN 'registrado'
    ELSE "estado"
  END
)::report_status;
ALTER TABLE "historial_estados" ALTER COLUMN "estado" SET NOT NULL;

CREATE OR REPLACE FUNCTION set_first_status_for_report()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO historial_estados (reporte_id, estado, motivo)
    VALUES (NEW.id, 'registrado', 'Reporte recien creado');

    RETURN NULL;  -- ignored for AFTER triggers
END;
$$ LANGUAGE plpgsql;
