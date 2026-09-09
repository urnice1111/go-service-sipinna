package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
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
func UploadToS3(uploader *S3Uploader) gin.HandlerFunc {
	return func(c *gin.Context) {
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid multipart form",
			})
			return
		}

		files := form.File["files"]
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No files provided",
			})
			return
		}

		if len(files) > 2 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "maximum 2 photos permited",
			})
			return
		}

		uploaded := make([]gin.H, 0, len(files))

		for _, file := range files {
			key := fmt.Sprintf(
				"uploads/%s",
				generateUniqueFilename(file.Filename),
			)

			url, err := uploader.Upload(
				c.Request.Context(),
				file,
				key,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":    "Upload failed",
					"filename": file.Filename,
					"details":  err.Error(),
					"uploaded": uploaded,
				})
				return
			}

			uploaded = append(uploaded, gin.H{
				"filename": file.Filename,
				"url":      url,
				"key":      key,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"files":   uploaded,
		})
	}
}
