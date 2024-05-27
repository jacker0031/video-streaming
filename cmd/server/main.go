package main

import (
	"github.com/gin-gonic/gin"
	"video-streaming/cmd/config"
	"video-streaming/pkg/database"
	"video-streaming/pkg/handlers"
)

func main() {
	// Initialize the database
	database.Init()

	config.Load()
	// Set up Gin router
	r := gin.Default()

	// Routes
	r.POST("/register", handlers.Register)
	r.POST("/login", handlers.Login)
	r.POST("/upload", handlers.Upload)
	r.GET("/videos", handlers.FetchVideo)

	// Start the server
	r.Run(":8080")
}
