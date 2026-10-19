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
	ChildrenQuantity *int       `json:"children_quantity" db:"cantidad_ninos"`
	ChildrenAge      string     `json:"children_age" db:"edad_ninos"`
	WorkType         string     `json:"work_type" db:"tipo_trabajo"`
	SightingTime     string     `json:"sighting_time" db:"horario_avistamiento"`
	Risk             string     `json:"risk" db:"riesgo"`
	ImmediateDanger  bool       `json:"immediate_danger" db:"peligro_inmediato"`
	Reference        string     `json:"reference" db:"referencia"`
	ZoneID           uuid.UUID  `json:"zone_id" db:"zona_id"`
	CasoID           uuid.UUID  `json:"case_id" db:"caso_id"`
	State            string     `json:"state" db:"-"`
	SuspiciusLevel   float64    `json:"suspicius_level" db:"sospechoso"`
	LLMAnalizedAt    time.Time  `json:"llm_analized_at" db:"llm_analizado_at"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// IndividualReport es un reporte tal como se muestra al personal en los listados:
// incluye el nombre de la zona, el del ciudadano y el último estado del historial.
//
// SuspiciusLevel es nil cuando el reporte aún no se analiza o cuando se oculta
// al ciudadano dueño del reporte.
type IndividualReport struct {
	CaseID           *uuid.UUID `json:"case_id"`
	CaseName         *string    `json:"case_name"`
	Folio            string     `json:"folio" db:"folio"`
	Description      string     `json:"description" db:"descripcion"`
	Latitude         float32    `json:"latitude" db:"latitud"`
	Longitude        float32    `json:"longitude" db:"longitud"`
	ChildrenQuantity *int       `json:"children_quantity" db:"cantidad_ninos"`
	WorkType         string     `json:"work_type" db:"tipo_trabajo"`
	CreatedtAt       time.Time  `json:"created_at" db:"created_at"`
	SuspiciusLevel   *float64   `json:"suspicius_level" db:"sospechoso"`
	ChildrenAge      string     `json:"children_age" db:"edad_ninos"`
	ZoneName         string     `json:"zone_name" db:"nombre_zona"`
	ZoneMunicipio    string     `json:"zone_municipio" db:"municipio_zona"`
	CitizenName      string     `json:"citizen_name" db:"nombre_ciudadano"`
	LastState        string     `json:"last_state" db:"ultimo_estado"`
	StateChangedAt   time.Time  `json:"state_changed_at" db:"estado_changed_at"`
	SightingTime     string     `json:"sighting_time" db:"horario_avistamiento"`
	Risk             string     `json:"risk" db:"riesgo"`
	ImmediateDanger  bool       `json:"immediate_danger" db:"peligro_inmediato"`
	Priority         string     `json:"priority" db:"prioridad"`
	Sex              string     `json:"sex" db:"sexo"`
	Reference        string     `json:"reference" db:"referencia"`
	// DuplicateOf es el folio del reporte del que este podría ser duplicado.
	DuplicateOf   string `json:"duplicate_of" db:"folio_duplicado"`
	ChildrenCared *int   `json:"children_cared" db:"nna_atendidos"`
}

// IndividualReportInfoBrief es el resumen de un reporte que ve el ciudadano en
// la lista de sus propios reportes (GET /report).
type IndividualReportInfoBrief struct {
	Folio          string    `json:"folio" db:"folio"`
	State          string    `json:"report_state" db:"report_state"`
	Latitude       float32   `json:"latitude" db:"latitud"`
	Longitude      float32   `json:"longitude" db:"longitud"`
	Description    string    `json:"description" db:"description"`
	CreatedAt      time.Time `json:"created_at"`
	StateChangedAt time.Time `json:"state_changed_at"`
}

// ReportDetail incluye expediente interno y fotos.
type ReportDetail struct {
	IndividualReport
	Responsible  string `json:"responsible"`
	ChanneledTo  string `json:"channeled_to"`
	FollowUpDate string `json:"follow_up_date"`
	Outcome      string `json:"outcome"`
	ClosedDate   string `json:"closed_date"`
	// Images son URLs prefirmadas de S3 con vigencia corta (15 minutos).
	Images []string `json:"images"`
}

// CaseFile actualiza el expediente: null conserva el valor; una cadena vacía lo borra.
// DuplicateOf recibe el folio original.
type CaseFile struct {
	Priority      *string `json:"priority"`
	Sex           *string `json:"sex"`
	Responsible   *string `json:"responsible"`
	ChanneledTo   *string `json:"channeled_to"`
	FollowUpDate  *string `json:"follow_up_date"`
	Outcome       *string `json:"outcome"`
	ChildrenCared *int    `json:"children_cared"`
	ClosedDate    *string `json:"closed_date"`
	DuplicateOf   *string `json:"duplicate_of"`
}

// ReportAction es una acción realizada sobre un reporte (tabla comentarios).
type ReportAction struct {
	ID        uuid.UUID `json:"id"`
	Text      string    `json:"text"`
	StaffName string    `json:"staff_name"`
	CreatedAt time.Time `json:"created_at"`
}

// PublicReportStatus contiene el estado visible sin sesión.
type PublicReportStatus struct {
	Folio          string    `json:"folio"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	StateChangedAt time.Time `json:"state_changed_at"`
}

// SubmittedReport contiene los datos para notificar el envío.
type SubmittedReport struct {
	ZoneID          uuid.UUID
	Folio           string
	AccessKey       uuid.UUID
	ImmediateDanger bool
}

// LogEntry es una fila de la bitácora de accesos.
type LogEntry struct {
	UserName  string    `json:"user_name"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

// ReportAssignment es el cuerpo de PATCH /report/:folio/assignment.
// Un campo null no cambia la columna.
type ReportAssignment struct {
	ZoneID   *string `json:"new_zone_id"`
	WorkType *string `json:"new_work_type"`
}
