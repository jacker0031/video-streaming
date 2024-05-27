package config

import (
	"github.com/spf13/viper"
	"log"
)

var (
	AWSRegion string
	S3Bucket  string
)

func Load() {
	viper.SetConfigName("config.yaml")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("cmd/config/")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	AWSRegion = viper.GetString("aws.region")
	S3Bucket = viper.GetString("aws.s3_bucket")
}
