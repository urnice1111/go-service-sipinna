package models

import (
	"github.com/google/uuid"
	"time"
)

type Case struct {
	CaseID      uuid.UUID  `json:"id" db:"id"`
	Name        string     `json:"name" db:"nombre"`
	Description string     `json:"description" db:"descripcion"`
	ZoneID      *uuid.UUID `json:"zone_id"`
	ReportCount int        `json:"report_count"`
	CreatedAt   time.Time  `json:"created_at"`
}

type CreateCase struct {
	Name        string   `json:"name" binding:"required,max=200"`
	Description string   `json:"description"`
	Folios      []string `json:"folios" binding:"required,min=1,dive,required"`
}

type UpdateCase struct {
	CaseID *uuid.UUID `json:"case_id"`
}

type UpdateResponse struct {
	Folio    string     `json:"folio" db:"folio"`
	CaseID   *uuid.UUID `json:"case_id" db:"case_id"`
	CaseName *string    `json:"case_name" db:"case_name"`
}
