package database

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"log"
	"video-streaming/pkg/models"
)

var DB *gorm.DB

func Init() {
	var err error
	DB, err = gorm.Open("sqlite3", "test.db")
	if err != nil {
		log.Fatal("failed to connect to database: ", err)
	}
	DB.AutoMigrate(&models.User{}, &models.Video{})
}
