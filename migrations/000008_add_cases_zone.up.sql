BEGIN;
ALTER TABLE casos
ADD COLUMN zona_id uuid REFERENCES zonas (id) ON DELETE RESTRICT;

CREATE INDEX idx_casos_zona ON casos (zona_id);
WITH zonas_por_caso AS (
    SELECT caso_id, (array_agg(DISTINCT zona_id))[1] AS zona_id
    FROM reportes
    WHERE caso_id IS NOT NULL
    GROUP BY caso_id
    HAVING COUNT(DISTINCT zona_id) = 1
       AND COUNT(zona_id) = COUNT(*)
)
UPDATE casos AS c
SET zona_id = z.zona_id
FROM zonas_por_caso AS z
WHERE c.id = z.caso_id;

COMMIT;