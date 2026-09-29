BEGIN;

CREATE TYPE estado_imagen AS ENUM (
    'pendiente',
    'registrado'
);

ALTER TABLE imagenes_reporte ADD estado estado_imagen DEFAULT 'pendiente';

COMMIT;