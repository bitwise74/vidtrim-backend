package api

import (
	"bitwise74/video-api/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (a *API) FileOwns(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	fileID := c.Param("id")
	if fileID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "No file ID provided",
			"requestID": requestID,
		})
		return
	}

	var owns bool
	err := a.DB.
		Model(model.File{}).
		Where("id = ? AND user_id = ?", fileID, userID).
		Select("count(*) > 0").
		Find(&owns).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error":     "File not found",
				"requestID": requestID,
			})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to check if user owns a file", zap.Error(err))
		return
	}

	if owns {
		c.JSON(http.StatusOK, gin.H{"owns": true})
		return
	}

	c.JSON(http.StatusForbidden, gin.H{"owns": false})
}
