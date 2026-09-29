CREATE TYPE report_status AS ENUM (
  'DRAFT',
  'registrado',
  'en_revision',
  'en_seguimiento',
  'canalizado',
  'concluido',
  'archivado',
  'cancelado',
  'reincidente'
);

ALTER TABLE "historial_estados" ALTER COLUMN "estado" TYPE report_status USING (
  CASE
    WHEN "estado" IS NULL OR "estado" IN ('pendiente', 'Reporte recibido') THEN 'DRAFT'
    ELSE "estado"
  END
)::report_status;
ALTER TABLE "historial_estados" ALTER COLUMN "estado" SET NOT NULL;

