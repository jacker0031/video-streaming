package s3

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"log"
	"mime"
	"mime/multipart"
	"path/filepath"
	"video-streaming/cmd/config"
)

func UploadFile(file multipart.File, filename string) (string, error) {
	// Initialize AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(config.AWSRegion),
	}))
	s3Uploader := s3manager.NewUploader(sess)

	// Determine the file's MIME type
	contentType := mime.TypeByExtension(filepath.Ext(filename))

	// Upload the file to S3
	result, err := s3Uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(config.S3Bucket),
		Key:         aws.String(filename),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		log.Printf("Failed to upload file to S3: %v", err)
		return "", err
	}

	// Return the URL of the uploaded file
	return result.Location, nil
}
