package models

import "github.com/google/uuid"

type Case struct {
	CaseID      uuid.UUID `db:"id"`
	Name        string    `db:"nombre"`
	Description string    `db:"descripcion"`
}

type CreateCase struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Folios      []string `json:"folios"`
}

type UpdateCase struct {
	CaseID *uuid.UUID `json:"case_id"`
}

type UpdateResponse struct {
	Folio    string     `json:"folio" db:"folio"`
	CaseID   *uuid.UUID `json:"case_id" db:"case_id"`
	CaseName *string    `json:"case_name" db:"case_name"`
}
