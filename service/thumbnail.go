// Package service contains stuff related to the background processing
// of the application
package service

import (
	"bitwise74/video-api/util"
	"context"
	"errors"
	"os"
	"time"

	"go.uber.org/zap"
)

// MakeThumbnail creates a thumbnail from a multipart.File
// TODO :Cleanup
func MakeThumbnail(temp *os.File, j *JobQueue, userID string, thumbPath string) error {
	zap.L().Debug("Creating thumbnail for video")

	jobID := util.RandStr(10)
	ProgressMap.Store(userID, FFMpegJobStats{
		Progress: 0,
		JobID:    jobID,
	})
	defer ProgressMap.Delete(userID)

	done := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	err := j.Enqueue(&FFmpegJob{
		ID:         jobID,
		UserID:     userID,
		FilePath:   temp.Name(),
		Args:       &[]string{"-loglevel", "error", "-ss", "0", "-i", temp.Name(), "-frames:v", "1", "-q:v", "2", "-vf", "scale=-640:360", thumbPath},
		Done:       done,
		CancelFunc: cancel,
		Ctx:        ctx,
		Opts:       nil,
	})
	if err != nil {
		return err
	}

	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return errors.New("request timed out")
	}

	zap.L().Debug("Done creating thumbnail")

	return nil
}
