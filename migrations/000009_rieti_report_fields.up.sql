-- Campos del reporte RIETI: riesgo, peligro inmediato, ubicación detallada,
-- contacto opcional para seguimiento, expediente interno y bitácora de accesos.

-- Estados intermedios entre "reporte" y "caso confirmado". Los valores nuevos no se
-- usan en esta migración (Postgres no lo permite en la misma transacción).
ALTER TYPE report_status ADD VALUE IF NOT EXISTS 'validado' AFTER 'en_revision';
ALTER TYPE report_status ADD VALUE IF NOT EXISTS 'caso_confirmado' AFTER 'validado';
ALTER TYPE report_status ADD VALUE IF NOT EXISTS 'en_atencion' AFTER 'canalizado';
ALTER TYPE report_status ADD VALUE IF NOT EXISTS 'duplicado';

ALTER TABLE reportes
  -- Lo captura el ciudadano
  ADD COLUMN riesgo                 varchar,  -- 'si' | 'no' | 'no_se'
  ADD COLUMN peligro_inmediato      boolean NOT NULL DEFAULT false,
  ADD COLUMN colonia                varchar,
  ADD COLUMN referencia             text,
  -- Contacto opcional para recibir el folio; se borra al cerrar el reporte
  ADD COLUMN contacto_email         varchar,
  ADD COLUMN contacto_telefono      varchar,
  -- Llave para consultar el estado público del folio sin cuenta
  ADD COLUMN clave_consulta         uuid NOT NULL DEFAULT gen_random_uuid(),
  -- Expediente interno
  ADD COLUMN prioridad              varchar NOT NULL DEFAULT 'normal',  -- 'normal' | 'alta'
  ADD COLUMN sexo                   varchar,
  ADD COLUMN posible_duplicado_de   uuid REFERENCES reportes (id) ON DELETE SET NULL,
  ADD COLUMN responsable            varchar,
  ADD COLUMN dependencia_canalizada varchar,
  ADD COLUMN fecha_seguimiento      date,
  ADD COLUMN resultado              text,
  ADD COLUMN nna_atendidos          int,
  ADD COLUMN fecha_cierre           date;

CREATE INDEX idx_reportes_duplicado ON reportes (posible_duplicado_de);

-- Quién consultó o modificó información de reportes y del personal.
CREATE TABLE bitacora (
  id          bigserial PRIMARY KEY,
  usuario_id  uuid REFERENCES usuarios (id) ON DELETE SET NULL,
  metodo      varchar NOT NULL,
  ruta        varchar NOT NULL,
  estatus     int NOT NULL,
  ip          varchar,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_bitacora_created_at ON bitacora (created_at DESC);
