package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var progressMap = sync.Map{}

func (a *API) FFmpegProgress(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	if _, ok := progressMap.Load(userID); !ok {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error":     "No running jobs found",
			"requestID": requestID,
		})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "nocache")
	c.Header("Connection", "keep-alive")

	ticker := time.NewTicker(time.Millisecond * 200)
	defer ticker.Stop()
	defer progressMap.Delete(userID)

	for range ticker.C {
		val, ok := progressMap.Load(userID)
		if !ok {
			continue
		}

		v := val.(FFMpegJobStats)

		fmt.Fprintf(c.Writer, "data: %.2f\n\n", v.Progress)
		c.Writer.Flush()

		if v.Progress >= 100 {
			break
		}
	}

	fmt.Fprintf(c.Writer, "data: %.2f\n\n", 100.0)
	c.Writer.Flush()
}
