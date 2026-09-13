-- La app manda la direccion en texto y la condicion del niño,
-- pero la tabla original no tenia esas columnas
ALTER TABLE "reportes" ADD COLUMN IF NOT EXISTS "direccion" varchar;
ALTER TABLE "reportes" ADD COLUMN IF NOT EXISTS "condicion" varchar;
