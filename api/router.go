// Package api contains all endpoints available
package api

import (
	"bitwise74/video-api/aws"
	"bitwise74/video-api/db"
	"bitwise74/video-api/middleware"
	"bitwise74/video-api/security"
	"bitwise74/video-api/service"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	cache "github.com/chenyahui/gin-cache"
	"github.com/chenyahui/gin-cache/persist"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var store = persist.NewMemoryStore(time.Minute)

type API struct {
	DB       *gorm.DB
	Router   *gin.Engine
	Argon    *security.ArgonHash
	S3       *aws.S3Client
	JobQueue *service.JobQueue
	Uploader *service.Uploader
}

func NewRouter() (*API, error) {
	a := &API{
		JobQueue: service.NewJobQueue(),
		Router:   gin.New(),
	}

	db, err := db.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SQLite database, %w", err)
	}
	a.DB = db

	origins := strings.Split(os.Getenv("HOST_CORS"), ",")

	a.Router.Use(
		cors.New(cors.Config{
			AllowOrigins:     origins,
			AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "TurnstileToken", "Range"},
			ExposeHeaders:    []string{"Content-Length", "Content-Range"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}),
		gin.Recovery(),
		middleware.NewRequestIDMiddleware(),
		gin.Logger(),
	)

	a.Router.HandleMethodNotAllowed = true
	a.Router.RedirectFixedPath = true

	// maxUploadSize, _ := strconv.ParseInt(os.Getenv("UPLOAD_MAX_SIZE"), 10, 64)
	rateLimit, _ := strconv.Atoi(os.Getenv("SECURITY_RATE_LIMIT"))

	jwt := middleware.NewJWTMiddleware(db)
	turnstile := middleware.NewTurnstileMiddleware()
	rateLimiter := middleware.RateLimiterMiddleware(middleware.RateLimiterConfig{
		RequestsPerSecond: rateLimit,
		Burst:             rateLimit * 2,
		CleanupInterval:   time.Second,
	})

	main := a.Router.Group("/api", rateLimiter)
	{
		// HEAD /api/heartbeat 		-> Used to check if the server is alive
		main.HEAD("/heartbeat", a.Heartbeat)

		// HEAD /api/validate		-> Validates a JWT token
		main.GET("/validate", jwt, a.Validate)
	}

	users := main.Group("/users")
	{
		// GET /api/users		-> Returns the basic info of a user
		users.GET("", jwt, a.UserFetch)

		// POST /api/users 		-> Registers a new user
		users.POST("", a.UserRegister)

		// POST /api/users/login 	-> Logs in a user and returns a JWT token
		users.POST("/login", a.UserLogin)

		// POST /api/users/verify	-> Verifies a new user
		users.POST("/verify", a.UserVerify)

		// DELETE /api/users/:id 	-> Deletes a user account
		// users.DELETE("/:id", jwt)
	}

	files := main.Group("/files", jwt)
	{
		// GET /api/files/:id/owns	-> Checks if a user owns a file
		files.GET("/:id/owns", cacheFor(5*60), a.FileOwns)

		// GET /api/files/:id		-> Returns a file by it's ID if the user owns it
		files.GET("/:id", a.FileFetch)

		// GET /api/files/bulk 		-> Returns a user's files in bulk
		files.GET("/bulk", a.FileFetchBulk)

		// POST /api/files         	-> Uploads a new file and stores it in the database
		files.POST("", a.FileUpload)

		// PATCH /api/files/:id		-> Updates a file
		files.PATCH("/:id", a.FileEdit)

		// DELETE /api/files/:id	-> Deletes a file owned by a user
		files.DELETE("/:id", a.FileDelete)

		// GET /api/files/search	-> Searches for files saved in the database
		files.GET("/search", cacheFor(15), a.FileSearch)
	}

	ffmpeg := main.Group("/ffmpeg", jwt)
	{
		// GET /api/ffmpeg/start	-> Starts an FFmpeg job
		ffmpeg.GET("/start", a.FFMpegStart)

		// GET /api/ffmpeg/progress	-> Returns the progress of a job
		ffmpeg.GET("/progress", a.FFmpegProgress)

		// POST /api/ffmpeg/process	-> Processes a file provided in a multipart form
		ffmpeg.POST("/process", turnstile, a.FFmpegProcess)
	}

	a.Argon = security.New()
	s3, err := aws.NewS3()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client, %w", err)
	}

	a.S3 = s3
	a.JobQueue.StartWorkerPool()

	a.Uploader = service.NewUploader(a.JobQueue, s3)

	return a, nil
}

// TODO: use redis instead
func cacheFor(sec int) gin.HandlerFunc {
	return cache.CacheByRequestURI(store, time.Second*time.Duration(sec))
}
