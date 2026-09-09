CREATE OR REPLACE FUNCTION set_first_status_for_report()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO historial_estados (reporte_id, estado, motivo)
    VALUES (NEW.id,'Reporte recibido', 'Reporte recien creado');

    RETURN NULL;  -- ignored for AFTER triggers
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_first_status_for_report
AFTER INSERT ON reportes
FOR EACH ROW
EXECUTE FUNCTION set_first_status_for_report();