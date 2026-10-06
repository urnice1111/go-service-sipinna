package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"go-service-sipinna/internal/models"
	"go-service-sipinna/internal/repository"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// S3Uploader handles uploads to AWS S3
type S3Uploader struct {
	client     *s3.Client
	bucketName string
}

// NewS3Uploader creates a new S3 uploader
func NewS3Uploader(bucketName string) (*S3Uploader, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &S3Uploader{
		client:     s3.NewFromConfig(cfg),
		bucketName: bucketName,
	}, nil
}

func generateUniqueFilename(originalName string) string {
	ext := filepath.Ext(originalName)

	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomStr := hex.EncodeToString(randomBytes)

	timestamp := time.Now().Format("20060102-150405")

	return fmt.Sprintf("%s-%s%s", timestamp, randomStr, ext)
}

// Upload sends a file to S3
func (u *S3Uploader) Upload(ctx context.Context, file *multipart.FileHeader, key string) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	// Detect content type
	buffer := make([]byte, 512)
	n, err := src.Read(buffer)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	contentType := http.DetectContentType(buffer[:n])

	// Reset reader position
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to reset file: %w", err)
	}

	// Upload to S3
	_, err = u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(u.bucketName),
		Key:         aws.String(key),
		Body:        src,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Return the S3 URL
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", u.bucketName, key), nil
}


// UploadToS3 handles multiple S3 uploads.
func UploadToS3(uploader *S3Uploader, pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var response models.ResponseS3
		fmt.Println("CONTENT TYPE:", c.GetHeader("Content-Type")) //debu

		file, err := c.FormFile("file")

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err})
		}
		reportID := c.Param("report_id")

		if reportID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No reportID provided"})
			return
		}

		imageID := c.Param("image_id")
		if imageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No reportID provided"})
			return
		}

		image, err := repository.GetImage(pool, reportID, imageID)

		if image.Status == "registrado" {
			response.Status = "already_uploaded"
			response.ImageID = image.URL
			response.Succes = true
			response.Message = "Image was already uploaded"

			c.JSON(http.StatusOK, response)
			return

		}

		url, err := uploader.Upload(
			c.Request.Context(),
			file,
			image.URL,
		)

		if err != nil {
			response.Status = "upload_failed"
			response.Message = "Failed to upload: " + err.Error()
			response.Succes = false
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		err = repository.UpdateImageStatus(pool, imageID)

		if err != nil {
			response.Status = "uploaded_not_updated"
			response.ImageID = ""
			response.Message = "Succesfully inserted into s3 but not updated on DB"
			response.Succes = false

			c.JSON(http.StatusInternalServerError, response)
			return

		}

		response.Status = "uploaded"
		response.ImageID = url
		response.Message = "Succesfully inserted into s3"
		response.Succes = true

		c.JSON(http.StatusOK, response)
	}
}
