package api

import (
	"bitwise74/video-api/model"
	"context"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type deleteInfo struct {
	S3Key string
	Size  int
}

func (a *API) FileDelete(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	fileID := c.Param("id")
	if fileID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "ID is missing",
			"requestID": requestID,
		})
		return
	}

	var info deleteInfo

	err := a.DB.
		Model(model.File{}).
		Where("user_id = ? AND id = ?", userID, fileID).
		Select("s3_key, size + COALESCE(thumbnail_size, 0) AS size").
		First(&info).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error":     "File not found. It either doesn't exist or you don't own it",
				"requestID": requestID,
			})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to check if file exists", zap.Error(err))
		return
	}

	err = a.DB.
		Where("s3_key IN ?", []string{info.S3Key, "thumb_" + info.S3Key}).
		Delete(model.File{}).
		Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to check if file exists", zap.Error(err))
		return
	}

	resp, err := a.S3.C.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: a.S3.Bucket,
		Delete: &types.Delete{
			Objects: []types.ObjectIdentifier{
				{
					Key: &info.S3Key,
				},
				{
					Key: aws.String("thumbnail_" + info.S3Key),
				},
			},
		},
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to delete file from S3", zap.Error(err))
		return
	}

	for _, v := range resp.Deleted {
		zap.L().Debug("Deleted item", zap.String("item", *v.Key))
	}

	err = a.DB.
		Model(model.Stats{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{
			"used_storage":   gorm.Expr("used_storage - ?", info.Size),
			"uploaded_files": gorm.Expr("uploaded_files - ?", 1),
		}).
		Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to decrement user's used storage", zap.Error(err))
		return
	}

	var newStats model.Stats

	err = a.DB.
		Where("user_id = ?", userID).
		First(&newStats).
		Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to decrement user's used storage", zap.Error(err))
		return
	}

	c.JSON(http.StatusOK, newStats)
}
