package models

// ImagesRequest es el cuerpo de POST /report/:report_id/images: la lista de imágenes
// que el cliente va a subir para un reporte. Por cada una se registra una fila
// pendiente en imagenes_reporte antes de subir el archivo a S3.
type ImagesRequest struct {
	Images []ImageInfo
}

// ImageInfo describe una imagen que el cliente quiere subir.
type ImageInfo struct {
	// FileName es el nombre original del archivo en el dispositivo.
	FileName string `json:"file_name"`
	// ContentType es el tipo MIME (p. ej. "image/jpeg"); de él se toma la extensión
	// de la llave en S3.
	ContentType string `json:"content_type"`
}

// GetImage es una fila de imagenes_reporte.
type GetImage struct {
	// URL es la llave del objeto en el bucket de S3 (p. ej. "report/20260101-120000-abcd.jpeg").
	URL string `json:"image_url" db:"url"`
	// Status es "pendiente" hasta que el archivo se sube a S3 y luego "registrado".
	Status string `json:"status" db:"estado"`
}

// ResponseS3 es la respuesta de PUT /report/:report_id/images/:image_id.
type ResponseS3 struct {
	// Succes indica si la imagen quedó subida y registrada.
	Succes bool `json:"success"`
	// Status es un código legible por máquina: "uploaded", "already_uploaded",
	// "image_not_found", "upload_failed" o "uploaded_not_updated".
	Status string `json:"status"`
	// ImageID contiene la URL o llave de la imagen en S3 cuando la subida tuvo éxito.
	ImageID string `json:"image_id"`
	// Message es una descripción para humanos del resultado.
	Message string `json:"message"`
}
