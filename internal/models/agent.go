package models

import "github.com/google/uuid"

type ElevenLabsRequest struct {
	ReportID     *uuid.UUID `db:"id"`
	Description  string     `json:"description"`
	Place        string     `json:"place"`
	NNAQuantity  int        `json:"children_quantity"`
	NNAAge       string     `json:"children_age"`
	WorkType     string     `json:"work_type"`
	SightingTime string     `json:"sighting_time"`
}

type GeoCode struct {
	Latitude  float32 `json:"latitude"`
	Longitude float32 `json:"longitude"`
}
