CREATE INDEX IF NOT EXISTS idx_historial_reporte_changed
  ON historial_estados (reporte_id, changed_at DESC);