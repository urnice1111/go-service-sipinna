package models

type ImagesRequest struct {
	Images []ImageInfo
}

type ImageInfo struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
}

type GetImage struct {
	URL    string `json:"image_url" db:"url"`
	Status string `json:"status" db:"estado"`
}

type ResponseS3 struct {
	Succes  bool   `json:"success"`
	Status  string `json:"status"`
	ImageID string `json:"image_id"`
	Message string `json:"message"`
}
