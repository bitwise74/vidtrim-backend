package api

import (
	"bitwise74/video-api/model"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type fileEditOpts struct {
	Name string `json:"name"`
	// TODO: add ffmpeg opts
}

func (a *API) FileEdit(c *gin.Context) {
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

	var data fileEditOpts
	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "Malformed or invalid JSON request body",
			"requestID": requestID,
		})

		zap.L().Error("Failed to read JSON body", zap.Error(err))
		return
	}

	if strings.TrimSpace(data.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "No new name provided",
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

	file.OriginalName = data.Name

	err = a.DB.
		Updates(file).
		Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to update file entry", zap.Error(err))
		return
	}

	c.JSON(http.StatusOK, file)
}
