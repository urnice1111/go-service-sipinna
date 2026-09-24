-- reportes.estado es la fuente de verdad; cada cambio queda registrado en historial_estados.
-- cambiado_por y motivo vienen de set_config('app.admin_id' / 'app.motivo', ..., true) en la
-- transacción del backend. Si el cambio se hace a mano (p. ej. en Supabase) quedan en NULL.
CREATE OR REPLACE FUNCTION log_report_status_change()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO historial_estados (reporte_id, estado, cambiado_por, motivo)
    VALUES (
        NEW.id,
        NEW.estado,
        NULLIF(current_setting('app.admin_id', true), '')::uuid,
        NULLIF(current_setting('app.motivo', true), '')
    );

    RETURN NULL;  -- ignored for AFTER triggers
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER log_report_status_change
AFTER UPDATE OF estado ON reportes
FOR EACH ROW
WHEN (OLD.estado IS DISTINCT FROM NEW.estado)
EXECUTE FUNCTION log_report_status_change();

-- Reportes cuyo estado se cambió antes de que existiera este trigger: se registra su estado
-- actual para que el historial coincida con reportes.estado.
INSERT INTO historial_estados (reporte_id, estado, motivo)
SELECT r.id, r.estado, 'Sincronizado: estado cambiado antes de registrar historial'
FROM reportes AS r
LEFT JOIN LATERAL (
    SELECT h.estado
    FROM historial_estados AS h
    WHERE h.reporte_id = r.id
    ORDER BY h.changed_at DESC, h.id DESC
    LIMIT 1
) AS he ON TRUE
WHERE he.estado IS DISTINCT FROM r.estado;
