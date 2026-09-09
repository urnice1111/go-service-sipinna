package models

import (
	"time"

	"github.com/google/uuid"
)

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
	State            string     `json:"state" db:"estado"`
	SuspiciusLevel   float64    `json:"suspicius_level" db:"sospechoso"`
	LLMAnalizedAt    time.Time  `json:"llm_analized_at" db:"llm_analizado_at"`
	DeleteDate       time.Time  `json:"delete_date" db:"fecha_eliminacion_programada"`
	CreatedAt        time.Time  `json:"created_at" db: "created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

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
