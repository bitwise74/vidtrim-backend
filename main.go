package main

import (
	"bitwise74/video-api/api"
	"bitwise74/video-api/config"
	"bitwise74/video-api/service"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	err := config.Setup()
	if err != nil {
		panic(err)
	}

	a, err := api.NewRouter()
	if err != nil {
		panic(err)
	}

	zap.L().Info("Server starting")

	// Check for useless tokens every day because they expire rarely
	go service.TokenCleanup(time.Hour*24, a.DB)

	// Check for expired accounts rarely because they have a week to verify
	go service.AccountCleanup(time.Hour*24*7, a.DB, a.S3.C)

	err = a.Router.Run(":" + os.Getenv("HOST_PORT"))
	if err != nil {
		panic(err)
	}
}
