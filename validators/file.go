package validators

import (
	"bitwise74/video-api/model"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

var (
	ErrFileTooLarge        = errors.New("file too large")
	ErrFileNameTooLong     = errors.New("file name is too long")
	ErrFileTypeUnsupported = errors.New("unsupported file type")
	ErrNoFile              = errors.New("no file provided")
	ErrNoSpace             = errors.New("not enough space")
)

const maxFileNameSize = 245 // Takes into account the thumbnail_ prefix

func FileValidator(fh *multipart.FileHeader, db *gorm.DB, userID string) (int, multipart.File, error) {
	if fh == nil {
		return http.StatusBadRequest, nil, ErrNoFile
	}

	// Check headers first which is easy to spoof, but faster for legit clients
	ct := fh.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "video/") {
		return http.StatusBadRequest, nil, ErrFileTypeUnsupported
	}

	if len(fh.Filename) > maxFileNameSize {
		return http.StatusBadRequest, nil, ErrFileNameTooLong
	}

	maxFileSize := viper.GetInt64("upload.max_size")
	if fh.Size > maxFileSize {
		return http.StatusRequestEntityTooLarge, nil, ErrFileTooLarge
	}

	if db != nil {
		var usedSpace int64
		err := db.
			Model(model.Stats{}).
			Where("user_id = ? ", userID).
			Select("used_storage").
			First(&usedSpace).
			Error
		if err != nil {
			return http.StatusInternalServerError, nil, err
		}

		if usedSpace+fh.Size > viper.GetInt64("storage.max_storage") {
			return http.StatusConflict, nil, ErrNoSpace
		}
	}

	// And now do the checks on the actual file to avoid
	// malicious clients
	f, err := fh.Open()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	mime, err := mimetype.DetectReader(f)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	if !mime.Is("video/mp4") {
		return http.StatusBadRequest, nil, ErrFileTypeUnsupported
	}

	_, err = f.Seek(maxFileSize+1, io.SeekStart)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	buf := make([]byte, 1)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return http.StatusInternalServerError, nil, err
	}

	if n > 0 {
		return http.StatusRequestEntityTooLarge, nil, ErrFileTooLarge
	}

	f.Seek(0, io.SeekStart)

	return 0, f, nil
}
