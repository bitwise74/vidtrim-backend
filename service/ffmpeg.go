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
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type FFmpegJob struct {
	ID       string
	UserID   string
	FilePath string
	Output   io.Writer
	Opts     *validators.ProcessingOptions
	Args     *[]string
	Ctx      context.Context
	Done     chan error
}

type FFMpegJobStats struct {
	Progress float64
	JobID    string
}

type JobQueue struct {
	jobs    chan *FFmpegJob
	threads int
	workers int64
}

// NewJobQueue initializes a new job queue that limits the
// max amount of jobs that can be queued at once
func NewJobQueue() *JobQueue {
	maxJobs, _ := strconv.ParseInt(os.Getenv("FFMPEG_MAX_JOBS"), 10, 32)
	workers, _ := strconv.ParseInt(os.Getenv("FFMPEG_WORKERS"), 10, 32)

	return &JobQueue{
		jobs:    make(chan *FFmpegJob, maxJobs),
		threads: getThreadsPerJob(workers),
		workers: workers,
	}
}

// Figures out the amount of threads to use per ffmpeg job
func getThreadsPerJob(c int64) int {
	totalCores := runtime.NumCPU()
	threads := max(int(math.Floor(float64(totalCores)/float64(c))), 1)

	zap.L().Debug("Threads per ffmpeg job specified", zap.Int("threads", threads), zap.Int64("workers", c))
	return threads
}

func (q *JobQueue) StartWorkerPool() {
	for range q.workers {
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

func (q *JobQueue) MakeFFmpegFlags(opts *validators.ProcessingOptions, p string) ([]string, float64, error) {
	args := []string{}

	useGPU, _ := strconv.ParseBool(os.Getenv("FFMPEG_USE_GPU"))
	hwaccel := os.Getenv("FFMPEG_HWACCEL")
	encoder := os.Getenv("FFMPEG_ENCODER")

	if encoder == "" {
		encoder = "libx264"
	}

	var duration float64
	var err error

	if useGPU && hwaccel != "" {
		args = append(args, "-hwaccel", hwaccel)
	}

	args = append(args, "-i", p)

	if opts.TrimStart > 0 {
		args = append(args, "-ss", util.FloatToTimestamp(opts.TrimStart))
	}

	if opts.TrimEnd > 0 && opts.TrimStart >= 0 {
		args = append(args, "-to", util.FloatToTimestamp(opts.TrimEnd))
		duration = float64(opts.TrimEnd - opts.TrimStart)
	} else {
		duration, err = GetDuration(p)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to run ffprobe to determine video duration: %w", err)
		}
	}

	args = append(args, "-c:v", encoder, "-threads", strconv.Itoa(q.threads))

	if opts.TargetSize > 0 {
		totalKilobits := opts.TargetSize * 8388.608
		totalBitrateKbps := totalKilobits / duration
		videoBitrateKbps := totalBitrateKbps - 128
		if videoBitrateKbps <= 0 {
			videoBitrateKbps = 5
		}
		videoBitrateStr := fmt.Sprintf("%.0fK", videoBitrateKbps)
		bufSizeStr := fmt.Sprintf("%dk", int(videoBitrateKbps*2))
		args = append(args,
			"-b:v", videoBitrateStr,
			"-maxrate", videoBitrateStr,
			"-bufsize", bufSizeStr,
		)
	}

	preset := opts.ProcessingSpeed
	if strings.Contains(encoder, "nvenc") {
		switch preset {
		case "ultrafast", "superfast":
			preset = "fast"
		case "veryfast":
			preset = "medium"
		case "faster":
			preset = "hp"
		default:
			preset = "fast"
		}
	} else if preset == "" {
		preset = "fast"
	}
	args = append(args, "-preset", preset)

	args = append(args,
		"-c:a", "copy",
		"-movflags", "+frag_keyframe+empty_moov+faststart",
		"-loglevel", "error",
	)

	args = append(args,
		"-f", "mp4",
		"pipe:1",
		"-progress", "pipe:2",
		"-nostats",
	)

	return args, duration, nil
}

func (q *JobQueue) runFFmpegJob(job *FFmpegJob) error {
	defer ProgressMap.Delete(job.UserID)
	var duration float64
	var err error

	if job.Args == nil {
		if job.Opts == nil {
			return errors.New("no arguments provided")
		}

		var args []string

		args, duration, err = q.MakeFFmpegFlags(job.Opts, job.FilePath)
		if err != nil {
			return err
		}

		job.Args = &args
	}

	cmd := exec.CommandContext(job.Ctx, "ffmpeg", *job.Args...)

	zap.L().Debug("Running FFmpeg command", zap.String("cmd", cmd.String()))

	stderrPipe, _ := cmd.StderrPipe()
	defer stderrPipe.Close()

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
	defer stdout.Close()

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
