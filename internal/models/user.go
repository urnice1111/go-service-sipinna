package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID `json:"id" db:"id"`
	Name            string    `json:"nombre" db:"nombre"`
	Role            string    `json:"rol" db:"rol"`
	Age             int       `json:"edad" db:"edad"`
	Genre           string    `json:"genero" db:"genero"`
	Email           *string   `json:"email" db:"email"`
	TelephoneNumber *string   `json:"telefono" db:"telefono"`
	HashedPassword  string    `json:"-" db:"password_hash"`
	ZoneID          *string   `json:"zona_id,omitempty" db:"zona_id"`
	AccountState    string    `json:"estado_cuenta" db:"estado_cuenta"`
	CreatedAt       time.Time `json:"created_at" db: "created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}
