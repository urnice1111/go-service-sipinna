package models

import "github.com/google/uuid"

// Zone es una zona geográfica de atención (tabla zonas). Cada reporte se asigna a la
// zona con coordenadas más cercana y cada alimentador atiende una sola zona.
type Zone struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"nombre"`
	Municipio string    `json:"municipio" db:"municipio"`
	// nil when the zone has no coordinates in the db.
	Latitude  *float64 `json:"latitude" db:"latitude"`
	Longitude *float64 `json:"longitude" db:"longitude"`
}
