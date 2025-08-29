package api

import (
	"bitwise74/video-api/model"
	"bitwise74/video-api/service"
	"bitwise74/video-api/util"
	"bitwise74/video-api/validators"
	"context"
	"io"
	"net/http"
	"os"
	"path"
	"time"

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
	defer f.Close()

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

	tempProcessed, err := os.CreateTemp("", "processed-*.mp4")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to create temporary processed file", zap.String("requestID", requestID), zap.Error(err))
		return
	}
	defer tempProcessed.Close()
	defer os.Remove(tempProcessed.Name())

	var ffmpegOpts []string
	var useGPU bool

	if path.Ext(fh.Filename) == ".mkv" {
		ffmpegOpts = append(ffmpegOpts,
			"-y",
			"-i", temp.Name(),
			"-movflags", "+faststart",
			"-f", "mp4",
			tempProcessed.Name(),
		)
		useGPU = true
	} else {
		ffmpegOpts = append(ffmpegOpts,
			"-y",
			"-i", temp.Name(),
			"-c:a", "copy",
			"-c:v", "copy",
			"-movflags", "+faststart",
			"-f", "mp4",
			tempProcessed.Name(),
		)
	}

	done := make(chan error, 1)

	ctxReq := c.Request.Context()
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctx, cancelMerged := util.MergeContexts(ctxReq, ctxTimeout)
	defer cancelMerged()

	err = a.JobQueue.Enqueue(&service.FFmpegJob{
		ID:       util.RandStr(5),
		UserID:   userID,
		FilePath: temp.Name(),
		Output:   tempProcessed,
		UseGPU:   useGPU,
		Args:     &ffmpegOpts,
		Ctx:      ctx,
		Done:     done,
	})
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":     "Job queue is full. Please wait a moment before trying again",
			"requestID": requestID,
		})

		zap.L().Warn("FFmpeg job queue is full")
		return
	}

	select {
	case err := <-done:
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})
			return
		}
	case <-ctx.Done():
		c.JSON(http.StatusRequestTimeout, gin.H{
			"error":     "Request was cancelled or timed out",
			"requestID": requestID,
		})

		zap.L().Warn("Request context done before FFmpeg finished", zap.Error(ctx.Err()))
		return
	}

	fileEnt, err := a.Uploader.Do(tempProcessed.Name(), fh.Filename, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
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
