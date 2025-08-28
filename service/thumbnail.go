// Package service contains stuff related to the background processing
// of the application
package service

import (
	"bitwise74/video-api/util"
	"context"
	"os"
	"path"
	"time"

	"go.uber.org/zap"
)

// MakeThumbnail creates a thumbnail from a multipart.File
// TODO :Cleanup
func MakeThumbnail(input string, j *JobQueue, userID string) (p string, err error) {
	zap.L().Debug("Creating thumbnail for video")

	jobID := util.RandStr(10)
	ProgressMap.Store(userID, FFMpegJobStats{
		Progress: 0,
		JobID:    jobID,
	})

	done := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()

	thumbPath := path.Join(os.TempDir(), util.RandStr(10)+".webp")
	zap.L().Debug("Writing thumbnail file", zap.String("path", thumbPath))

	err = j.Enqueue(&FFmpegJob{
		ID:     jobID,
		UserID: userID,
		Args:   &[]string{"-loglevel", "error", "-ss", "0", "-i", input, "-frames:v", "1", "-q:v", "2", "-vf", "scale=-640:360", thumbPath},
		Done:   done,
		Ctx:    ctx,
		Opts:   nil,
	})
	if err != nil {
		return "", err
	}

	select {
	case err := <-done:
		if err != nil {
			return "", err
		}
		// case <-ctx.Done():
		// 	return "", nil
	}

	return thumbPath, nil
}
