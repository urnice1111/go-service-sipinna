
BEGIN;

ALTER TABLE imagenes_reporte DROP COLUMN estado;


DROP TYPE IF EXISTS estado_imagen;

COMMIT;