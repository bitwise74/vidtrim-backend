// Package service contains stuff related to the background processing
// of the application
package service

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"go.uber.org/zap"
)

// MakeThumbnail creates a thumbnail from a multipart.File
func MakeThumbnail(temp *os.File, dest string) error {
	zap.L().Debug("Creating thumbnail for video")
	now := time.Now()

	// -ss before the input seeks to the first millisecond before the file opens
	// (uses key-frame seeking so that it's faster)
	cmd := exec.Command("ffmpeg", "-loglevel", "error", "-ss", "0", "-i", temp.Name(), "-frames:v", "1", "-q:v", "2", "-vf", "scale=-1:320", dest, "-y")

	err := cmd.Run()
	if err != nil {
		// TODO: add ffmpeg error dump
		return fmt.Errorf("failed to create thumbnail for video, %w", err)
	}

	zap.L().Debug("Finished creating thumbnail", zap.Duration("took", time.Since(now)))

	return nil
}
