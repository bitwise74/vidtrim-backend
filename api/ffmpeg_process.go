package api

import (
	"bitwise74/video-api/service"
	"bitwise74/video-api/validators"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (a *API) FFmpegProcess(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	jobID := c.Query("jobID")
	userID := c.MustGet("userID").(string)

	if _, ok := service.ProgressMap.Load(userID); !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Invalid job ID",
			"requestID": requestID,
		})
		return
	}

	var opts validators.ProcessingOptions
	if err := c.Bind(&opts); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Failed to read multipart body",
			"requestID": requestID,
		})

		zap.L().Error("Failed to read multipart body", zap.Error(err))
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to open multipart file", zap.Error(err))
		return
	}

	if code, err := validators.ProcessingOptsValidator(&opts, fh); err != nil {
		c.AbortWithStatusJSON(code, gin.H{
			"error":     err.Error(),
			"requestID": requestID,
		})
		return
	}

	code, f, err := validators.FileValidator(fh, nil, "")
	if err != nil {
		if code == http.StatusInternalServerError {
			zap.L().Error("Failed to validate file", zap.Error(err))

			err = errors.New("internal server error")
		}

		c.AbortWithStatusJSON(code, gin.H{
			"error":     err.Error(),
			"requestID": requestID,
		})
		return
	}

	temp, err := os.CreateTemp("", "upload-*.mp4")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to create temporary file", zap.Error(err))
		return
	}
	defer os.Remove(temp.Name())

	_, err = io.Copy(temp, f)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to copy data to temporary file", zap.Error(err))
		return
	}

	f.Seek(0, io.SeekStart)

	c.Header("Content-Type", "video/mp4")
	c.Header("Transfer-Encoding", "chunked")

	ctxReq := c.Request.Context()
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctx, cancelMerged := mergeContexts(ctxReq, ctxTimeout)
	defer cancelMerged()

	done := make(chan error, 1)

	// Enqueue can only error if the queue is full
	err = a.JobQueue.Enqueue(&service.FFmpegJob{
		ID:         jobID,
		UserID:     userID,
		FilePath:   temp.Name(),
		Output:     c.Writer,
		Opts:       &opts,
		Ctx:        ctx,
		CancelFunc: cancelMerged,
		Done:       done,
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error":     "Job queue is full. Please wait a moment before trying again",
			"requestID": requestID,
		})

		zap.L().Warn("FFmpeg job queue is full")
		return
	}

	select {
	case err := <-done:
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})

			zap.L().Error("FFmpeg job failed", zap.Error(err))
			return
		}
	case <-ctx.Done():
		c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{
			"error":     "Request was cancelled or timed out",
			"requestID": requestID,
		})

		zap.L().Warn("Request context done before FFmpeg finished", zap.Error(ctx.Err()))
		return
	}

	close(done)
}

func mergeContexts(ctx1, ctx2 context.Context) (context.Context, context.CancelFunc) {
	merged, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-ctx1.Done():
			cancel()
		case <-ctx2.Done():
			cancel()
		case <-merged.Done():
		}
	}()

	return merged, cancel
}
