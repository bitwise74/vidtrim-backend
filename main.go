package main

import (
	"bitwise74/video-api/api"
	"bitwise74/video-api/config"

	"github.com/gin-gonic/gin"
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

	a.Router.Run(":8080")
}
