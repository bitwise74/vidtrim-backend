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
	r2Key string
	size  int
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
		Select("r2_key, size").
		Find(info).
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
		Where("r2_key = ?", info.r2Key).
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

	_, err = a.R2.C.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: a.R2.Bucket,
		Delete: &types.Delete{
			Objects: []types.ObjectIdentifier{
				{
					Key: &info.r2Key,
				},
				{
					Key: aws.String("thumbnail_" + info.r2Key),
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

	err = a.DB.
		Model(model.Stats{}).
		Where("user_id = ?", userID).
		Update("used_storage", gorm.Expr("used_storage - ?", info.size)).
		Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to decrement user's used storage", zap.Error(err))
		return
	}

	c.Status(http.StatusOK)
}
