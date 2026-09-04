package models

import "github.com/google/uuid"

type Zone struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"nombre"`
	Municipio string    `json:"municipio" db:"municipio"`
	Latitude  float32   `json:"latitude" db:"latitude"`
	Longitude float32   `json:"longitude" db:"longitude"`
}
