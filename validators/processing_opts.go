package validators

import (
	"errors"
	"mime/multipart"
	"net/http"
	"slices"
)

var validProcessingSpeeds = []string{"ultrafast", "superfast", "faster", "fast", "medium"}

type ProcessingOptions struct {
	File            *multipart.FileHeader `form:"file"`
	TrimStart       float64               `form:"trimStart"`
	TrimEnd         float64               `form:"trimEnd"`
	TargetSize      float64               `form:"targetSize"`
	ProcessingSpeed string                `form:"processingSpeed"`
	SaveToCloud     bool                  `form:"saveToCloud"`
}

// ProcessingOptsValidator needs the file header to check if the target size is bigger than the actual video size
func ProcessingOptsValidator(o *ProcessingOptions, fSize float64) (code int, err error) {
	if o.TrimStart > o.TrimEnd {
		return http.StatusBadRequest, errors.New("trim start can't be bigger than trim end")
	}

	if o.TrimStart == o.TrimEnd {
		return http.StatusBadRequest, errors.New("trim start and trim end can't be the same")
	}

	if o.TargetSize != 0 && o.TargetSize == fSize || o.TargetSize > fSize {
		return http.StatusBadRequest, errors.New("invalid target size provided")
	}

	if !slices.Contains(validProcessingSpeeds, o.ProcessingSpeed) {
		return http.StatusBadRequest, errors.New("invalid processing speed option")
	}

	return 0, nil
}
