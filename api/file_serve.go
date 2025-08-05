package api

import (
	"bitwise74/video-api/model"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// FileServe serves a file for viewing on a webiste or in a browser directly from the CDN
func (a *API) FileServe(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	fileID := c.Param("fileID")
	if fileID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "No file ID provided",
			"requestID": requestID,
		})
		return
	}

	thumbStr := c.DefaultQuery("t", "1")
	thumb, err := strconv.ParseBool(thumbStr)
	if err != nil {
		thumb = true
	}

	var r2ID string

	err = a.DB.
		Model(model.File{}).
		Where("id = ?", fileID).
		Select("r2_key").
		Find(&r2ID).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

		zap.L().Error("Failed to check if file exists", zap.String("id", fileID), zap.Error(err))
		return
	}

	cType := "video/mp4"

	if thumb {
		r2ID = "thumb_" + r2ID
		cType = "image/webp"
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	}

	input := &s3.GetObjectInput{
		Bucket: a.R2.Bucket,
		Key:    aws.String(r2ID),
	}

	result, err := a.R2.C.GetObject(context.Background(), input)
	if err != nil {
		zap.L().Error("Failed to get file", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "File not found",
			"requestID": requestID,
		})
		return
	}
	defer result.Body.Close()

	c.Header("Content-Type", cType)

	_, err = io.Copy(c.Writer, result.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to copy file buffer to context writer", zap.String("id", fileID), zap.Error(err))
		return
	}
}
