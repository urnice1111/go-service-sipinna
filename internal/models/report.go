package models

import (
	"time"

	"github.com/google/uuid"
)

type Report struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	Folio         string     `json:"folio" db:"folio"`
	UserID        *uuid.UUID `json:"usuario_id" db:"usuario_id"` // nil si el reporte es anonimo
	Description   string     `json:"descripcion" db:"descripcion"`
	Latitude      float64    `json:"latitud" db:"latitud"`
	Longitude     float64    `json:"longitud" db:"longitud"`
	Address       string     `json:"direccion" db:"direccion"`
	ChildrenCount int        `json:"cantidad_ninos" db:"cantidad_ninos"`
	ChildrenAge   string     `json:"edad_ninos" db:"edad_ninos"`
	WorkType      string     `json:"tipo_trabajo" db:"tipo_trabajo"`
	SightingTime  string     `json:"horario_avistamiento" db:"horario_avistamiento"`
	Condition     string     `json:"condicion" db:"condicion"`
	State         string     `json:"estado" db:"estado"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}
