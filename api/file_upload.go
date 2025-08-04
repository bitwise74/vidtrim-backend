package api

import (
	"bitwise74/video-api/model"
	"bitwise74/video-api/service"
	"bitwise74/video-api/util"
	"bitwise74/video-api/validators"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const tenMB = 10 << 20

func (a *API) FileUpload(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	if !strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Invalid request, use multipart/form-data",
			"requestID": requestID,
		})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to parse multipart form", zap.Error(err))
		return
	}

	files := form.File["file"]
	if len(files) <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "No file provided",
			"requestID": requestID,
		})
		return
	}

	fh := files[0]

	ext := path.Ext(fh.Filename)
	fh.Filename = util.RandStr(10) + ext

	code, f, err := validators.FileValidator(fh, a.DB, userID)
	if err != nil {
		if code == http.StatusInternalServerError {
			zap.L().Error("Failed to validate file", zap.Error(err))

			// That's to set the error into a general one for the users
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
	r2Key := uuid.NewString()

	errChan := make(chan error, 3)
	uploadedIDs := make([]string, 2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Make and upload the thumbnail in the background
	go func() {
		zap.L().Debug("Creating thumbnail for uploaded video")

		thumbName := "thumb_" + r2Key
		thumbPath := path.Join(os.TempDir(), thumbName)

		err := service.MakeThumbnail(temp, thumbPath)
		if err != nil {
			errChan <- fmt.Errorf("failed to create thumbnail, %w", err)
			return
		}

		file, err := os.Open(thumbPath)
		if err != nil {
			errChan <- fmt.Errorf("failed to open dest file, %w", err)
			return

		}

		stat, err := file.Stat()

		if err != nil {
			errChan <- fmt.Errorf("failed to stat dest file, %w", err)
			return
		}

		_, err = a.R2.C.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        a.R2.Bucket,
			Key:           &fh.Filename,
			Body:          file,
			ContentType:   aws.String("image/webp"),
			ContentLength: aws.Int64(stat.Size()),
		})
		if err != nil {
			errChan <- fmt.Errorf("failed to upload thumbnail to R2, %w", err)
			return
		}

		errChan <- nil
		uploadedIDs = append(uploadedIDs, thumbName)
	}()

	// Upload file to R2
	go func() {
		var err error

		if fh.Size > tenMB {
			u := manager.NewUploader(a.R2.C, nil)

			_, err = u.Upload(ctx, &s3.PutObjectInput{
				Bucket:        a.R2.Bucket,
				Key:           &r2Key,
				Body:          f,
				ContentLength: &fh.Size,
				ContentType:   aws.String(fh.Header.Get("Content-Type")),
			})
			if err != nil {
				errChan <- fmt.Errorf("failed to upload file to R2, %w", err)
				return
			}
		} else {
			_, err = a.R2.C.PutObject(ctx, &s3.PutObjectInput{
				Bucket: a.R2.Bucket,
				Key:    &r2Key,
				Body:   f,
			})
			if err != nil {
				errChan <- fmt.Errorf("failed to upload file to R2, %w", err)
				return
			}
		}

		errChan <- nil
		uploadedIDs = append(uploadedIDs, r2Key)
	}()

	// Get video duration and save stuff to DB
	go func() {
		duration, err := service.GetDuration(temp.Name())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})

			zap.L().Error("Failed to get video duration", zap.Error(err))
			return
		}

		fileRecord := model.File{
			UserID:       userID,
			R2Key:        r2Key,
			OriginalName: fh.Filename,
			Size:         fh.Size,
			Format:       fh.Header.Get("Content-Type"),
			CreatedAt:    time.Now().Unix(),
			Duration:     duration,
			State:        "ready",
		}

		err = a.DB.WithContext(ctx).Create(&fileRecord).Error
		if err != nil {
			zap.L().Error("Failed to insert DB entry", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})
			return
		}
	}()

	for range len(errChan) {
		err := <-errChan
		if err != nil {
			cancel()

			zap.L().Error("Background operation failed", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})

			// Cleanup any potential uploads
			if len(uploadedIDs) != 0 {
				for _, id := range uploadedIDs {
					_, err := a.R2.C.DeleteObject(context.Background(), &s3.DeleteObjectInput{
						Bucket: a.R2.Bucket,
						Key:    aws.String(id),
					})
					if err != nil {
						zap.L().Error("Failed to cleanup after failed upload", zap.Error(err))
						return
					}

					zap.L().Debug("Cleaned up after failed upload", zap.String("id", id))
				}
			}

			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"key": r2Key,
	})
}
