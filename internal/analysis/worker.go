package analysis

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	workerInterval  = 15 * time.Second
	workerBatchSize = 20
)

// Worker revisa periódicamente los reportes enviados (que ya no son borrador)
// que aún no tienen puntaje (columna "sospechoso" en NULL), los evalúa y guarda el resultado.
// Corre en segundo plano, así que crear un reporte nunca espera a Jev.
type Worker struct {
	pool      *pgxpool.Pool
	evaluator *Evaluator
}

// NewWorker crea el proceso. Si jev es nil (no hay TYPESAFE_API_KEY), el análisis queda desactivado.
func NewWorker(pool *pgxpool.Pool, jev *JevClient) *Worker {
	ev := &Evaluator{}
	if jev != nil {
		ev.Jev = jev // se asigna solo si no es nil para que la interfaz quede realmente vacía
	}
	return &Worker{pool: pool, evaluator: ev}
}

// Run procesa reportes pendientes hasta que se cancele ctx.
//
// Sin API key de Jev el proceso no arranca: los reportes se quedan pendientes
// (sospechoso en NULL) para que los analice el servidor que sí tiene la clave.
// Así, un backend local sin clave no "gana" los reportes con un puntaje de solo reglas.
func (w *Worker) Run(ctx context.Context) {
	if w.evaluator.Jev == nil {
		log.Println("[analisis] TYPESAFE_API_KEY no configurada: análisis desactivado en este servidor, los reportes quedan pendientes")
		return
	}
	log.Println("[analisis] análisis de reportes con Jev activado")

	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()

	for {
		if stop := w.processBatch(ctx); stop {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Reportes sin puntaje, con las señales que usan las reglas.
// Las comparaciones de "mismo usuario" solo aplican a reportes con cuenta:
// que varias personas reporten el mismo lugar es normal y no es sospechoso.
const queryPendingReports = `
SELECT
	r.id,
	COALESCE(r.descripcion, ''),
	COALESCE(r.tipo_trabajo, ''),
	COALESCE(r.edad_ninos, ''),
	COALESCE(r.cantidad_ninos, 0),
	COALESCE(r.horario_avistamiento, ''),
	(SELECT COUNT(*) FROM imagenes_reporte AS i WHERE i.reporte_id = r.id),
	r.ciudadano_id IS NULL,
	(SELECT COUNT(*) FROM reportes AS o
		WHERE r.ciudadano_id IS NOT NULL
			AND o.ciudadano_id = r.ciudadano_id
			AND o.id <> r.id
			AND o.created_at BETWEEN r.created_at - INTERVAL '24 hours' AND r.created_at
			AND ABS(o.latitud - r.latitud) < 0.002
			AND ABS(o.longitud - r.longitud) < 0.002),
	(SELECT COUNT(*) FROM reportes AS o
		WHERE r.ciudadano_id IS NOT NULL
			AND o.ciudadano_id = r.ciudadano_id
			AND o.id <> r.id
			AND o.created_at BETWEEN r.created_at - INTERVAL '1 hour' AND r.created_at)
FROM reportes AS r
WHERE r.sospechoso IS NULL
	-- Los borradores (DRAFT) aún no tienen fotos ni datos finales: se analizan hasta que se envían
	AND COALESCE((
		SELECT h.estado::text
		FROM historial_estados AS h
		WHERE h.reporte_id = r.id
		ORDER BY h.changed_at DESC, h.id DESC
		LIMIT 1
	), 'DRAFT') <> 'DRAFT'
ORDER BY r.created_at
LIMIT $1
`

const queryUpdateSuspicion = `
UPDATE reportes
SET sospechoso = $2,
	llm_analizado_at = CASE WHEN $3::boolean THEN NOW() ELSE llm_analizado_at END
WHERE id = $1
`

type pendingReport struct {
	id    uuid.UUID
	input ReportInput
}

// processBatch analiza un lote. Regresa true si hay que detener el proceso
// (la cuenta de TypeSafe no funciona y reintentar no lo va a arreglar).
func (w *Worker) processBatch(ctx context.Context) bool {
	pending, err := w.loadPending(ctx)
	if err != nil {
		log.Printf("[analisis] no se pudieron leer reportes pendientes: %v", err)
		return false
	}

	for _, p := range pending {
		evalCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		res, err := w.evaluator.Evaluate(evalCtx, p.input)
		cancel()

		if err != nil {
			switch {
			case IsAccountError(err):
				// Clave inválida o sin saldo: no se guarda nada y los reportes quedan pendientes
				log.Printf("[analisis] la cuenta de Jev no funciona (clave inválida o sin saldo), análisis detenido; corrige TYPESAFE_API_KEY y reinicia el servidor: %v", err)
				return true
			case IsRetryable(err):
				// Jev saturado o sin red: el reporte queda pendiente y se intenta en el siguiente ciclo
				log.Printf("[analisis] jev no disponible, se reintentará: %v", err)
				return false
			default:
				// Jev rechazó este reporte en particular (p. ej. 422): se guarda el puntaje de reglas
				log.Printf("[analisis] jev rechazó el reporte %s, se usan solo reglas: %v", p.id, err)
			}
		}

		if err := w.save(ctx, p.id, res); err != nil {
			log.Printf("[analisis] no se pudo guardar el puntaje del reporte %s: %v", p.id, err)
			continue
		}
		log.Printf("[analisis] reporte %s: sospecha %.2f (jev=%t) motivos: %s",
			p.id, res.Score, res.UsedLLM, motivos(res.Reasons))
	}
	return false
}

func motivos(reasons []string) string {
	if len(reasons) == 0 {
		return "ninguno"
	}
	return strings.Join(reasons, "; ")
}

func (w *Worker) loadPending(ctx context.Context) ([]pendingReport, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := w.pool.Query(queryCtx, queryPendingReports, workerBatchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []pendingReport
	for rows.Next() {
		var p pendingReport
		var photos, nearby, lastHour int64
		if err := rows.Scan(
			&p.id,
			&p.input.Description,
			&p.input.WorkType,
			&p.input.ChildrenAge,
			&p.input.ChildrenQuantity,
			&p.input.SightingTime,
			&photos,
			&p.input.Anonymous,
			&nearby,
			&lastHour,
		); err != nil {
			return nil, err
		}
		p.input.PhotoCount = int(photos)
		p.input.SameUserNearby24h = int(nearby)
		p.input.SameUserLastHour = int(lastHour)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (w *Worker) save(ctx context.Context, id uuid.UUID, res Result) error {
	saveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := w.pool.Exec(saveCtx, queryUpdateSuspicion, id, res.Score, res.UsedLLM)
	return err
}
