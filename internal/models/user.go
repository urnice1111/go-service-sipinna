package models

import (
	"time"

	"github.com/google/uuid"
)

// User es una cuenta de la tabla usuarios. Según la tabla que la complemente es un
// ciudadano (ciudadanos) o personal (admins, con rol "administrador" o "alimentador").
//
// Email y TelephoneNumber son opcionales, pero toda cuenta tiene al menos uno.
// HashedPassword es un hash bcrypt y nunca se serializa a JSON; está vacío en las
// cuentas creadas con Google.
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
