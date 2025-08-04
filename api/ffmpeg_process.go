package api

import (
	"bitwise74/video-api/validators"
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (a *API) FFmpegProcess(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	jobID := c.Query("jobID")
	userID := c.MustGet("userID").(string)

	if _, ok := progressMap.Load(userID); !ok {
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

	command := []string{
		"-i",
		temp.Name(),
	}

	var duration float64

	if opts.TrimEnd != -1 && opts.TrimStart != -1 {
		command = append(command,
			"-ss", floatToTimestamp(opts.TrimStart),
			"-to", floatToTimestamp(opts.TrimEnd),
		)

		duration = float64(opts.TrimEnd - opts.TrimStart)
	} else {
		duration, err = getVideoDuration(temp.Name())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})

			zap.L().Error("Failed to run ffprobe to get video duration", zap.Error(err))
			return
		}
	}

	command = append(command,
		"-c:v", "libx264",
	)

	if opts.TargetSize > 0 {
		totalKilobits := opts.TargetSize * 8388.608
		totalBitrateKbps := totalKilobits / float64(duration)

		videoBitrateKbps := totalBitrateKbps - 128

		if videoBitrateKbps <= 0 {
			videoBitrateKbps = 5
		}

		videoBitrateStr := fmt.Sprintf("%.0fK", videoBitrateKbps)

		bufSizeKbps := int(videoBitrateKbps * 2)
		bufSizeStr := fmt.Sprintf("%dk", bufSizeKbps)

		command = append(command,
			"-b:v", videoBitrateStr,
			"-maxrate", videoBitrateStr,
			"-bufsize", bufSizeStr,
		)
	}
	if opts.ProcessingSpeed != "" {
		command = append(command, "-preset", opts.ProcessingSpeed)
	}

	command = append(command,
		"-movflags", "frag_keyframe+empty_moov",
		"-c:a", "aac",
		"-b:a", "128k",
	)

	command = append(command, "-f", "mp4", "pipe:1", "-progress", "pipe:2", "-nostats")
	cmd := exec.Command("ffmpeg", command...)

	zap.L().Debug("Running FFmpeg command", zap.String("cmd", cmd.String()))

	stderrP, _ := cmd.StderrPipe()

	go func() {
		scanner := bufio.NewScanner(stderrP)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "progress=end" {
				progressMap.Store(userID, FFMpegJobStats{
					JobID:    jobID,
					Progress: 100.0,
				})
				return
			}

			if after, ok := strings.CutPrefix(line, "out_time_ms="); ok {
				msStr := after
				outTimeMs, err := strconv.ParseFloat(msStr, 64)
				if err == nil {
					percent := (outTimeMs / (duration * 1000)) / 10
					progressMap.Store(userID, FFMpegJobStats{
						JobID:    jobID,
						Progress: percent,
					})
				}
			}
		}
		progressMap.Store(userID, FFMpegJobStats{
			JobID:    jobID,
			Progress: 100.0,
		})
	}()

	c.Header("Content-Type", "video/mp4")
	c.Header("Transfer-Encoding", "chunked")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create stdout pipe: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		c.String(http.StatusInternalServerError, "Failed to start FFmpeg: %v", err)
		return
	}

	_, err = io.Copy(c.Writer, stdout)
	if err != nil {
		c.String(500, "Streaming error: %v", err)
		return
	}

	cmd.Wait()
}

func getVideoDuration(path string) (float64, error) {
	command := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	zap.L().Debug("Running ffprobe command", zap.String("cmd", command.String()))
	output, err := command.CombinedOutput()
	if err != nil {
		// Dump ffprobe stderr to Linux stderr
		fmt.Fprintln(os.Stderr, string(output))
		return 0, err
	}

	var duration float64
	_, err = fmt.Sscanf(string(output), "%f", &duration)
	if err != nil {
		return 0, err
	}

	return duration, nil
}

// Formats float seconds into HH:MM:SS.MMM
func floatToTimestamp(seconds float64) string {
	seconds = math.Round(seconds*1000) / 1000

	wholeSeconds := int64(seconds)
	milliseconds := int((seconds - float64(wholeSeconds)) * 1000)

	hours := wholeSeconds / 3600
	remaining := wholeSeconds % 3600
	minutes := remaining / 60
	secs := remaining % 60

	result := fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, milliseconds)
	return result
}
