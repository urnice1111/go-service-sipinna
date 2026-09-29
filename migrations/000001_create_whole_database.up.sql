BEGIN;

CREATE TYPE account_status AS ENUM ('pendiente', 'activada');
CREATE TYPE admin_role     AS ENUM ('alimentador', 'administrador');

-- ---------------------------------------------------------
-- Función para mantener updated_at al día
-- ---------------------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------
-- Catálogos (van primero porque otras tablas las referencian)
-- ---------------------------------------------------------
CREATE TABLE zonas (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  nombre     varchar NOT NULL,
  municipio  varchar,
  latitude   decimal(9,6),
  longitude  decimal(9,6)
);

CREATE TABLE casos (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  nombre       varchar NOT NULL,
  descripcion  text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------
-- Usuarios
-- ---------------------------------------------------------
CREATE TABLE usuarios (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  nombre         varchar,
  telefono       varchar UNIQUE,
  email          varchar UNIQUE,
  password_hash  varchar,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ciudadanos (
  id      uuid PRIMARY KEY REFERENCES usuarios (id) ON DELETE CASCADE,
  edad    int,
  genero  varchar
);

CREATE TABLE admins (
  id             uuid PRIMARY KEY REFERENCES usuarios (id) ON DELETE CASCADE,
  rol            admin_role NOT NULL,
  -- RESTRICT: no se puede borrar una zona que tenga admins.
  zona_id        uuid REFERENCES zonas (id) ON DELETE RESTRICT,
  estado_cuenta  account_status NOT NULL DEFAULT 'pendiente'
);

CREATE TABLE otp_verificaciones (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  usuario_id   uuid NOT NULL REFERENCES usuarios (id) ON DELETE CASCADE,
  codigo_hash  varchar NOT NULL,
  tipo         varchar,
  estado       varchar,
  expira_at    timestamptz NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------
-- Reportes
-- ---------------------------------------------------------
CREATE SEQUENCE reportes_folio_seq START WITH 1;

CREATE TABLE reportes (
  id                            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  folio                         varchar UNIQUE,
  -- SET NULL: si el ciudadano borra su cuenta, el reporte se conserva anonimizado
  ciudadano_id                  uuid REFERENCES ciudadanos (id) ON DELETE SET NULL,
  descripcion                   text,
  latitud                       decimal(9,6),
  longitud                      decimal(9,6),
  cantidad_ninos                int,
  edad_ninos                    varchar,
  tipo_trabajo                  varchar,
  horario_avistamiento          varchar,
  -- RESTRICT: no se puede borrar una zona que tenga reportes
  zona_id                       uuid REFERENCES zonas (id) ON DELETE RESTRICT,
  -- SET NULL: borrar un caso solo desagrupa sus reportes
  caso_id                       uuid REFERENCES casos (id) ON DELETE SET NULL,
  sospechoso                    decimal,
  llm_analizado_at              timestamptz,
  fecha_eliminacion_programada  date,
  created_at                    timestamptz NOT NULL DEFAULT now(),
  updated_at                    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE imagenes_reporte (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reporte_id  uuid NOT NULL REFERENCES reportes (id) ON DELETE CASCADE,
  url         varchar NOT NULL,
  orden       int NOT NULL,
  UNIQUE (reporte_id, orden)
);

CREATE TABLE comentarios (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reporte_id  uuid NOT NULL REFERENCES reportes (id) ON DELETE CASCADE,
  -- SET NULL: si se borra el admin, el comentario se conserva
  admin_id    uuid REFERENCES admins (id) ON DELETE SET NULL,
  comentario  text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE historial_estados (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reporte_id    uuid NOT NULL REFERENCES reportes (id) ON DELETE CASCADE,
  estado        varchar NOT NULL,
  cambiado_por  uuid REFERENCES admins (id) ON DELETE SET NULL,
  motivo        text,
  changed_at    timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------
-- Índices en columnas FK (Postgres no los crea solo)
-- ---------------------------------------------------------
CREATE INDEX idx_admins_zona                ON admins (zona_id);
CREATE INDEX idx_otp_usuario                ON otp_verificaciones (usuario_id);
CREATE INDEX idx_reportes_ciudadano         ON reportes (ciudadano_id);
CREATE INDEX idx_reportes_zona              ON reportes (zona_id);
CREATE INDEX idx_reportes_caso              ON reportes (caso_id);
CREATE INDEX idx_comentarios_reporte        ON comentarios (reporte_id);
CREATE INDEX idx_comentarios_admin          ON comentarios (admin_id);
CREATE INDEX idx_historial_reporte          ON historial_estados (reporte_id);
CREATE INDEX idx_historial_cambiado_por     ON historial_estados (cambiado_por);

-- ---------------------------------------------------------
-- Triggers de updated_at
-- ---------------------------------------------------------
CREATE TRIGGER trg_usuarios_updated_at
  BEFORE UPDATE ON usuarios
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_casos_updated_at
  BEFORE UPDATE ON casos
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_reportes_updated_at
  BEFORE UPDATE ON reportes
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMIT;