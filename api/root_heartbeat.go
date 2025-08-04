package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *API) Heartbeat(c *gin.Context) {
	c.Status(http.StatusOK)
}
