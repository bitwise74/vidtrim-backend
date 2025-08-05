package service

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

func GetDuration(p string) (d float64, err error) {
	zap.L().Debug("Running FFprobe to determine video duration")

	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", p).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w (%s)", err, out)
	}

	d, err = strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("malformed duration: %w", err)
	}

	zap.L().Debug("FFprobe finished")
	return d, nil
}
