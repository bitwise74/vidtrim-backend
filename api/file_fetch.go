package api

import (
	"bitwise74/video-api/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (a *API) FileFetch(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "No file ID provided",
			"requestID": requestID,
		})

		return
	}

	var file model.File

	err := a.DB.
		Where("user_id = ? AND id = ?", userID, fileID).
		First(&file).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":     "File not found",
				"requestID": requestID,
			})

			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to fetch file from db", zap.Error(err))
		return
	}

	c.JSON(http.StatusOK, file)
}
