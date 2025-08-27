package api

import (
	"bitwise74/video-api/model"
	"bitwise74/video-api/util"
	"bitwise74/video-api/validators"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (a *API) FileUpload(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to open multipart file", zap.String("requestID", requestID), zap.Error(err))
		return
	}

	if fh == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":     "No file provided",
			"requestID": requestID,
		})
		return
	}

	code, f, err := validators.FileValidator(fh, a.DB, userID)
	if err != nil {
		c.JSON(code, gin.H{
			"error":     err.Error(),
			"requestID": requestID,
		})
		return
	}

	temp, err := os.CreateTemp("", "upload-*.mp4")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to create temporary file", zap.String("requestID", requestID), zap.Error(err))
		return
	}
	defer temp.Close()
	defer os.Remove(temp.Name())

	_, err = io.Copy(temp, f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to copy data to temporary file", zap.String("requestID", requestID), zap.Error(err))
		return
	}

	rand := util.RandStr(10)
	videoPath := path.Join(os.TempDir(), rand+".mp4")

	cmd := exec.CommandContext(context.Background(), "ffmpeg",
		"-y",
		"-i", temp.Name(),
		"-movflags", "+faststart",
		"-f", "mp4",
		videoPath,
	)

	var stdErr bytes.Buffer
	cmd.Stderr = &stdErr

	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "internal_server_error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to run ffmpeg", zap.Error(err), zap.String("stderr", stdErr.String()))
		return
	}

	fileEnt, err := a.Uploader.Do(videoPath, fh.Filename, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "internal_server_error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to upload video to S3", zap.Error(err))
		return
	}

	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&fileEnt).Error; err != nil {
			return err
		}

		if err := tx.
			Model(model.Stats{}).
			Where("user_id = ?", userID).
			Updates(map[string]any{
				"used_storage":   gorm.Expr("used_storage + ?", fileEnt.Size),
				"uploaded_files": gorm.Expr("uploaded_files + ?", 1),
			}).
			Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Database transaction failed", zap.String("requestID", requestID), zap.Error(err))
		return
	}

	c.JSON(http.StatusOK, fileEnt)
}
