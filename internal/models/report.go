package models

import (
	"time"

	"github.com/google/uuid"
)

// Report es un reporte de posible trabajo infantil (tabla reportes).
//
// SuspiciusLevel es el puntaje de sospecha calculado por el paquete analysis:
// 0 = parece legítimo, 1 = muy probablemente falso. State no se guarda en
// reportes; el estado actual es la fila más reciente de historial_estados.
type Report struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	Folio            string     `json:"folio" db:"folio"`
	CiudadanoID      *uuid.UUID `json:"citizen_id,omitempty" db:"ciudadano_id"`
	Description      string     `json:"description" db:"descripcion"`
	Latitude         float32    `json:"latitude" db:"latitud"`
	Longitude        float32    `json:"longitude" db:"longitud"`
	ChildrenQuantity int        `json:"children_quantity" db:"cantidad_ninos"`
	ChildrenAge      string     `json:"children_age" db:"edad_ninos"`
	WorkType         string     `json:"work_type" db:"tipo_trabajo"`
	SightingTime     string     `json:"sighting_time" db:"horario_avistamiento"`
	ZoneID           uuid.UUID  `json:"zone_id" db:"zona_id"`
	CasoID           uuid.UUID  `json:"case_id" db:"caso_id"`
	State            string     `json:"state" db:"-"`
	SuspiciusLevel   float64    `json:"suspicius_level" db:"sospechoso"`
	LLMAnalizedAt    time.Time  `json:"llm_analized_at" db:"llm_analizado_at"`
	DeleteDate       time.Time  `json:"delete_date" db:"fecha_eliminacion_programada"`
	CreatedAt        time.Time  `json:"created_at" db: "created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// IndividualReport es un reporte tal como se muestra al personal en los listados:
// incluye el nombre de la zona, el del ciudadano y el último estado del historial.
//
// SuspiciusLevel es nil cuando el reporte aún no se analiza o cuando se oculta
// al ciudadano dueño del reporte.
type IndividualReport struct {
	Folio            string    `json:"folio" db:"folio"`
	Description      string    `json:"description" db:"descripcion"`
	Latitude         float32   `json:"latitude" db:"latitud"`
	Longitude        float32   `json:"longitude" db:"longitud"`
	ChildrenQuantity int       `json:"children_quantity" db:"cantidad_ninos"`
	WorkType         string    `json:"work_type" db:"tipo_trabajo"`
	CreatedtAt       time.Time `json:"created_at" db:"created_at"`
	SuspiciusLevel   *float64  `json:"suspicius_level" db:"sospechoso"`
	ChildrenAge      string    `json:"children_age" db:"edad_ninos"`
	ZoneName         string    `json:"zone_name" db:"nombre_zona"`
	CitizenName      string    `json:"citizen_name" db:"nombre_ciudadano"`
	LastState        string    `json:"last_state" db:"ultimo_estado"`
	StateChangedAt   time.Time `json:"state_changed_at" db:"estado_changed_at"`
}

// IndividualReportInfoBrief es el resumen de un reporte que ve el ciudadano en
// la lista de sus propios reportes (GET /report).
type IndividualReportInfoBrief struct {
	Folio       string  `json:"folio" db:"folio"`
	State       string  `json:"report_state" db:"report_state"`
	Latitude    float32 `json:"latitude" db:"latitud"`
	Longitude   float32 `json:"longitude" db:"longitud"`
	Description string  `json:"description" db:"description"`
}

// ReportDetail es el detalle completo de un reporte (GET /report/:folio).
type ReportDetail struct {
	IndividualReport
	// SightingTime es el horario aproximado en que se vio a los menores.
	SightingTime string `json:"sighting_time"`
	// Images son URLs prefirmadas de S3 con vigencia corta (15 minutos).
	Images []string `json:"images"`
}
