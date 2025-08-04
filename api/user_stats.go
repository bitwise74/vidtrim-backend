package api

import (
	"bitwise74/video-api/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (a *API) UserStats(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	var stats model.Stats

	err := a.DB.
		Where("user_id = ?", userID).
		Find(&stats).
		Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to load user stats", zap.String("userID", userID), zap.Error(err))
		return
	}

	c.Header("Cache-Control", "max-age=3600")
	c.JSON(http.StatusOK, stats)
}
