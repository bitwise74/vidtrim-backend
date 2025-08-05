package main

import (
	"bitwise74/video-api/api"
	"bitwise74/video-api/config"

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

	err = a.Router.Run(":8080")
	if err != nil {
		panic(err)
	}
}
