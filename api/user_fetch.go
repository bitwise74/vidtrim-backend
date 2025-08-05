package api

import (
	"bitwise74/video-api/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type userFetchData struct {
	Videos []model.File `json:"videos"`
	Stats  model.Stats  `json:"stats"`
}

// UserFetch returns the first 20 videos and stats of a user.
// This is used when initially loading the dashboard
func (a *API) UserFetch(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	var videos []model.File

	err := a.DB.
		Where("user_id = ? AND r2_key NOT LIKE ?", userID, "thumb%").
		Limit(20).
		Find(&videos).
		Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to fetch initial user data", zap.Error(err))
		return
	}

	var stats model.Stats
	err = a.DB.
		Where("user_id = ?", userID).
		First(&stats).
		Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to fetch initial user data", zap.Error(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": videos,
		"stats":  stats,
	})
}
