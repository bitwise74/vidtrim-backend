package service

import (
	"bitwise74/video-api/util"
	"bitwise74/video-api/validators"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type FFmpegJob struct {
	ID         string
	UserID     string
	FilePath   string
	Output     io.Writer
	Opts       *validators.ProcessingOptions
	Ctx        context.Context
	CancelFunc context.CancelFunc
	Done       chan error
}

type FFMpegJobStats struct {
	Progress float64
	JobID    string
}

type JobQueue struct {
	jobs    chan *FFmpegJob
	threads int
}

// NewJobQueue initializes a new job queue that limits the
// max amount of jobs that can be queued at once
func NewJobQueue() *JobQueue {
	return &JobQueue{
		jobs:    make(chan *FFmpegJob, viper.GetInt("ffmpeg.max_jobs")),
		threads: getThreadsPerJob(viper.GetInt("ffmpeg.workers")),
	}
}

// Figures out the amount of threads to use per ffmpeg job
func getThreadsPerJob(c int) int {
	totalCores := runtime.NumCPU()
	threads := int(math.Floor(float64(totalCores) / float64(c)))

	if threads < 1 {
		threads = 1
	}

	zap.L().Debug("Figured out amount of threads to use", zap.Int("t", threads))
	return threads
}

func (q *JobQueue) StartWorkerPool() {
	for range viper.GetInt("ffmpeg.workers") {
		go q.worker()
	}
}

func (q *JobQueue) worker() {
	for job := range q.jobs {
		job.Done <- q.runFFmpegJob(job)
	}
}

func (q *JobQueue) Enqueue(job *FFmpegJob) error {
	select {
	case q.jobs <- job:
		return nil
	default:
		return errors.New("job queue full")
	}
}

func (q *JobQueue) runFFmpegJob(job *FFmpegJob) error {
	args := []string{
		"-i", job.FilePath,
	}

	var duration float64

	if job.Opts.TrimEnd > 1 && job.Opts.TrimStart != -1 {
		args = append(args,
			"-ss", util.FloatToTimestamp(job.Opts.TrimStart),
			"-to", util.FloatToTimestamp(job.Opts.TrimEnd),
		)

		duration = float64(job.Opts.TrimEnd - job.Opts.TrimStart)
	} else {
		var err error
		duration, err = GetDuration(job.FilePath)
		if err != nil {
			return fmt.Errorf("failed to run ffprobe to determine video duration: %w", err)
		}
	}

	args = append(args,
		"-c:v", "libx264",
		"-threads", strconv.Itoa(q.threads),
	)

	if job.Opts.TargetSize > 0 {
		totalKilobits := job.Opts.TargetSize * 8388.608
		totalBitrateKbps := totalKilobits / float64(duration)

		videoBitrateKbps := totalBitrateKbps - 128

		if videoBitrateKbps <= 0 {
			videoBitrateKbps = 5
		}

		videoBitrateStr := fmt.Sprintf("%.0fK", videoBitrateKbps)

		bufSizeKbps := int(videoBitrateKbps * 2)
		bufSizeStr := fmt.Sprintf("%dk", bufSizeKbps)

		args = append(args,
			"-b:v", videoBitrateStr,
			"-maxrate", videoBitrateStr,
			"-bufsize", bufSizeStr,
		)
	}

	if job.Opts.ProcessingSpeed != "" {
		args = append(args,
			"-preset", job.Opts.ProcessingSpeed,
		)
	}

	args = append(args,
		"-movflags", "frag_keyframe+empty_moov",
		"-c:a", "aac",
		"-b:a", "128k",
		"-loglevel", "error",
	)

	args = append(args,
		"-f", "mp4",
		"pipe:1", "-progress",
		"pipe:2", "-nostats",
	)

	cmd := exec.Command("ffmpeg", args...)

	zap.L().Debug("Running FFmpeg command", zap.String("cmd", cmd.String()))

	stderrPipe, _ := cmd.StderrPipe()
	stderrBuf := &bytes.Buffer{}

	go func() {
		scanner := bufio.NewScanner(io.TeeReader(stderrPipe, stderrBuf))
		for scanner.Scan() {
			line := scanner.Text()

			if line == "progress=end" {
				ProgressMap.Store(job.UserID, FFMpegJobStats{
					JobID:    job.ID,
					Progress: 100.0,
				})
				return
			}

			if after, ok := strings.CutPrefix(line, "out_time_ms="); ok {
				msStr := after
				outTimeMs, err := strconv.ParseFloat(msStr, 64)
				if err == nil {
					percent := (outTimeMs / (duration * 1000)) / 10
					ProgressMap.Store(job.UserID, FFMpegJobStats{
						JobID:    job.ID,
						Progress: percent,
					})
				}
			}
		}

		ProgressMap.Store(job.UserID, FFMpegJobStats{
			JobID:    job.ID,
			Progress: 100.0,
		})
	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg, %w", err)
	}

	_, err = io.Copy(job.Output, stdout)
	if err != nil {
		return fmt.Errorf("streaming error, %w", err)
	}

	if err := cmd.Wait(); err != nil {
		zap.L().Error("FFmpeg failed", zap.Error(err), zap.String("stderr", stderrBuf.String()))
		return fmt.Errorf("ffmpeg failed: %w", err)
	}

	return nil
}
