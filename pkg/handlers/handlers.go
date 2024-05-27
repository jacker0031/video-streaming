package handlers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"video-streaming/cmd/config"
	"video-streaming/pkg/auth"
	"video-streaming/pkg/database"
	"video-streaming/pkg/models"
	"video-streaming/pkg/s3"
)

type Credentials struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Register(c *gin.Context) {
	var creds Credentials
	if err := c.BindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	user := models.User{
		Username: creds.Username,
		Password: string(hashedPassword),
	}

	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "Account created"})
}

func Login(c *gin.Context) {
	var creds Credentials
	if err := c.BindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var user models.User
	if err := database.DB.Where("username = ?", creds.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(creds.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	token, err := auth.GenerateJWT(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func Upload(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
		return
	}

	token := authHeader[len("Bearer "):]
	claims, err := auth.ValidateJWT(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
	var user models.User
	if err := database.DB.Where("username = ?", claims.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}
	file, err := c.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Video file not found in form data"})
		return
	}

	tempDir := "tmp"

	if _, err := os.Stat(tempDir); err == nil {
		err = os.RemoveAll(tempDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to remove old temporary directory: %s", err)})
			return
		}
	}

	os.MkdirAll(tempDir, os.ModePerm)

	tempFilePath := filepath.Join(tempDir, file.Filename)
	outFile, err := os.Create(tempFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temporary file"})
		return
	}
	defer outFile.Close()

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
		return
	}
	defer src.Close()

	if _, err := io.Copy(outFile, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save uploaded file"})
		return
	}

	// Transcode video to HLS format
	hlsDir := filepath.Join(tempDir, "hls")
	os.MkdirAll(hlsDir, os.ModePerm)
	hlsFilePath := filepath.Join(hlsDir, "index.m3u8")
	cmd := exec.Command("ffmpeg", "-i", tempFilePath, "-c:v", "copy", "-c:a", "copy", "-start_number", "0", "-hls_time", "10", "-hls_list_size", "0", "-f", "hls", hlsFilePath)
	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transcode video to HLS format"})
		return
	}

	// Upload HLS files to S3
	hlsFiles, err := filepath.Glob(filepath.Join(hlsDir, "*"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list HLS files"})
		return
	}
	videoId := uuid.New().String()

	for _, filePath := range hlsFiles {
		fileName := filepath.Base(filePath)
		file, err := os.Open(filePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to open file %s", fileName)})
			return
		}
		defer file.Close()
		s3Key := fmt.Sprintf("%s/%s", videoId, fileName)
		if _, err := s3.UploadFile(file, s3Key); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload file %s to S3", fileName)})
			return
		}
	}

	// Save video metadata to the database
	video := models.Video{
		Title:       file.Filename,
		Description: "Default description",
		URL:         fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s/index.m3u8", config.S3Bucket, config.AWSRegion, videoId),
		UserID:      user.ID, // Replace with actual user ID from JWT
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := database.DB.Create(&video).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save video information to the database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Video uploaded and saved successfully", "video_url": video.URL})
}

func FetchVideo(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
		return
	}

	token := authHeader[len("Bearer "):]
	claims, err := auth.ValidateJWT(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}
	var user models.User
	if err := database.DB.Where("username = ?", claims.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	var videos []models.Video
	if err := database.DB.Where("user_id = ?", user.ID).Find(&videos).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"videos": videos})
}
