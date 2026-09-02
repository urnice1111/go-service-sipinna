CREATE TABLE "usuarios" (
  "id" uuid PRIMARY KEY,
  "nombre" varchar,
  "rol" varchar,
  "edad" int,
  "genero" varchar,
  "email" varchar UNIQUE,
  "telefono" varchar UNIQUE,
  "password_hash" varchar,
  "zona_id" uuid,
  "estado_cuenta" varchar,
  "created_at" timestamp DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamp DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "otp_verificaciones" (
  "id" uuid PRIMARY KEY,
  "usuario_id" uuid,
  "codigo_hash" varchar,
  "tipo" varchar,
  "estado" varchar,
  "expira_at" timestamp,
  "created_at" timestamp DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "zonas" (
  "id" uuid PRIMARY KEY,
  "nombre" varchar,
  "municipio" varchar
);

CREATE TABLE "casos" (
  "id" uuid PRIMARY KEY,
  "nombre" varchar,
  "descripcion" text,
  "created_at" timestamp DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamp DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "reportes" (
  "id" uuid PRIMARY KEY,
  "folio" varchar UNIQUE,
  "usuario_id" uuid,
  "descripcion" text,
  "latitud" decimal,
  "longitud" decimal,
  "cantidad_ninos" int,
  "edad_ninos" varchar,
  "tipo_trabajo" varchar,
  "horario_avistamiento" varchar,
  "zona_id" uuid,
  "caso_id" uuid,
  "estado" varchar,
  "sospechoso" boolean DEFAULT false,
  "llm_analizado_at" timestamp,
  "fecha_eliminacion_programada" date,
  "created_at" timestamp DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamp DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "imagenes_reporte" (
  "id" uuid PRIMARY KEY,
  "reporte_id" uuid,
  "url" varchar,
  "orden" int
);

CREATE TABLE "comentarios" (
  "id" uuid PRIMARY KEY,
  "reporte_id" uuid,
  "usuario_id" uuid,
  "comentario" text,
  "created_at" timestamp DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE "historial_estados" (
  "id" uuid PRIMARY KEY,
  "reporte_id" uuid,
  "estado" varchar,
  "cambiado_por" uuid,
  "motivo" text,
  "changed_at" timestamp DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX ON "imagenes_reporte" ("reporte_id", "orden");

COMMENT ON COLUMN "usuarios"."zona_id" IS 'Solo aplica a alimentador';

ALTER TABLE "usuarios" ADD FOREIGN KEY ("zona_id") REFERENCES "zonas" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "otp_verificaciones" ADD FOREIGN KEY ("usuario_id") REFERENCES "usuarios" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "reportes" ADD FOREIGN KEY ("usuario_id") REFERENCES "usuarios" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "reportes" ADD FOREIGN KEY ("zona_id") REFERENCES "zonas" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "reportes" ADD FOREIGN KEY ("caso_id") REFERENCES "casos" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "imagenes_reporte" ADD FOREIGN KEY ("reporte_id") REFERENCES "reportes" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "comentarios" ADD FOREIGN KEY ("reporte_id") REFERENCES "reportes" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "comentarios" ADD FOREIGN KEY ("usuario_id") REFERENCES "usuarios" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "historial_estados" ADD FOREIGN KEY ("reporte_id") REFERENCES "reportes" ("id") DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE "historial_estados" ADD FOREIGN KEY ("cambiado_por") REFERENCES "usuarios" ("id") DEFERRABLE INITIALLY IMMEDIATE;
