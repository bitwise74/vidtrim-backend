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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const multipartLimit = 100 << 20

func (a *API) FileUpload(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	if !strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Invalid request",
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
	// TODO: fix +faststart flag missing

	fh := files[0]

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
	s3Key := util.RandStr(7)

	errChan := make(chan error, 3)
	uploadedIDs := make([]string, 2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(3)

	var thumbSize int64 = 0

	// Make and upload the thumbnail in the background
	go func() {
		defer wg.Done()
		thumbName := "thumb_" + s3Key
		thumbPath := path.Join(os.TempDir(), thumbName) + ".webp"

		err = service.MakeThumbnail(temp, a.JobQueue, userID, thumbPath)
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

		_, err = a.S3.C.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        a.S3.Bucket,
			Key:           &thumbName,
			Body:          file,
			ContentType:   aws.String("image/webp"),
			ContentLength: aws.Int64(stat.Size()),
		})
		if err != nil {
			errChan <- fmt.Errorf("failed to upload thumbnail to S3, %w", err)
			return
		}

		errChan <- nil
		uploadedIDs = append(uploadedIDs, thumbName)
		thumbSize = stat.Size()

		err = a.DB.
			Create(&model.File{
				S3Key:        thumbName,
				UserID:       userID,
				OriginalName: thumbName + ".webp",
				Format:       "image/webp",
				CreatedAt:    time.Now().Unix(),
			}).
			Error
		if err != nil {
			errChan <- fmt.Errorf("failed to create database record, %w", err)
			return
		}

		errChan <- nil
	}()

	// Upload file to S3
	go func() {
		defer wg.Done()
		now := time.Now()

		var err error

		zap.L().Debug("Starting video upload")

		if fh.Size > multipartLimit {
			u := manager.NewUploader(a.S3.C, func(u *manager.Uploader) {
				u.Concurrency = 5
				u.PartSize = 5 << 20
			})

			_, err = u.Upload(ctx, &s3.PutObjectInput{
				Bucket:        a.S3.Bucket,
				Key:           &s3Key,
				Body:          f,
				ContentLength: &fh.Size,
				ContentType:   aws.String(fh.Header.Get("Content-Type")),
			})
			if err != nil {
				errChan <- fmt.Errorf("failed to upload file to S3, %w", err)
				return
			}
		} else {
			_, err = a.S3.C.PutObject(ctx, &s3.PutObjectInput{
				Bucket: a.S3.Bucket,
				Key:    &s3Key,
				Body:   f,
			})
			if err != nil {
				errChan <- fmt.Errorf("failed to upload file to S3, %w", err)
				return
			}
		}

		errChan <- nil
		uploadedIDs = append(uploadedIDs, s3Key)

		zap.L().Debug("File uploaded", zap.Duration("took", time.Since(now)))
	}()

	// Get video duration and save stuff to DB
	go func() {
		defer wg.Done()
		duration, err := service.GetDuration(temp.Name())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})

			zap.L().Error("Failed to get video duration", zap.Error(err))
			return
		}

		zap.L().Debug("Creating database entry for uploaded file")

		fileRecord := model.File{
			UserID:       userID,
			S3Key:        s3Key,
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

		errChan <- nil
	}()

	for range 3 {
		err := <-errChan
		if err != nil {
			cancel()

			zap.L().Error("Background operation failed", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})

			if len(uploadedIDs) != 0 {
				for _, id := range uploadedIDs {
					_, err := a.S3.C.DeleteObject(context.Background(), &s3.DeleteObjectInput{
						Bucket: a.S3.Bucket,
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

	// Don't let cancel run prematurely
	wg.Wait()

	// And after everything is done increment the amount of used storage
	err = a.DB.
		Model(model.Stats{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{
			"used_storage":   gorm.Expr("used_storage + ?", fh.Size+thumbSize),
			"uploaded_files": gorm.Expr("uploaded_files + ?", 1),
		}).
		Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to increment user's used storage", zap.Error(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"key": s3Key,
	})
}
