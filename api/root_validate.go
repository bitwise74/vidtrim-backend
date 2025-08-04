package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *API) Validate(c *gin.Context) {
	c.Status(http.StatusOK)
}
