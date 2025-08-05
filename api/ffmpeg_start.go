package api

import (
	"bitwise74/video-api/service"
	"bitwise74/video-api/util"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (a *API) FFMpegStart(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	if _, ok := service.ProgressMap.Load(userID); ok {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":     "A job is running already. Wait for it to finish first",
			"requestID": requestID,
		})
		return
	}

	jobID := util.RandStr(10)

	service.ProgressMap.Store(userID, service.FFMpegJobStats{
		Progress: 0.0,
		JobID:    jobID,
	})

	zap.L().Debug("Started a new FFmpeg job", zap.String("userID", userID), zap.String("jobID", jobID))

	// Delete the job after 3 minutes unless stopped early
	go func() {
		time.Sleep(time.Minute * 3)
		service.ProgressMap.Delete(userID)
	}()

	c.JSON(http.StatusOK, gin.H{
		"jobID": jobID,
	})
}
