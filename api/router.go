// Package api contains all endpoints available
package api

import (
	"bitwise74/video-api/aws"
	"bitwise74/video-api/db"
	"bitwise74/video-api/middleware"
	"bitwise74/video-api/security"
	"bitwise74/video-api/service"
	"fmt"
	"time"

	cache "github.com/chenyahui/gin-cache"
	"github.com/chenyahui/gin-cache/persist"
	ginzap "github.com/gin-contrib/zap"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
)

const (
	gray  = "\x1b[90m"
	reset = "\x1b[0m"
)

var store = persist.NewMemoryStore(time.Minute)

type API struct {
	DB       *gorm.DB
	Router   *gin.Engine
	Argon    *security.ArgonHash
	S3       *aws.S3Client
	JobQueue *service.JobQueue
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

	makeLogger()

	a.Router.Use(
		cors.New(cors.Config{
			AllowOrigins:     viper.GetStringSlice("host.cors"),
			AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "TurnstileToken", "Range"},
			ExposeHeaders:    []string{"Content-Length", "Content-Range"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}),
		gin.Recovery(),
		middleware.NewRequestIDMiddleware(),
		ginzap.GinzapWithConfig(zap.L(), &ginzap.Config{
			TimeFormat: "15:04:05.000",
			UTC:        true,
			Skipper: func(c *gin.Context) bool {
				return c.Request.Method == "HEAD"
			},
			Context: func(c *gin.Context) []zapcore.Field {
				fields := []zapcore.Field{}

				if v := c.GetString("requestID"); v != "" {
					fields = append(fields, zap.String("request_id", v))
				}

				if v := c.GetString("userID"); v != "" {
					fields = append(fields, zap.String("userID", v))
				}

				return fields
			},
		}),
	)

	a.Router.HandleMethodNotAllowed = true
	a.Router.RedirectFixedPath = true
	a.Router.MaxMultipartMemory = 5 << 20

	jwt := middleware.NewJWTMiddleware(db)
	turnstile := middleware.NewTurnstileMiddleware()
	maxUploadSize := viper.GetInt64("upload.max_size")
	rateLimiter := middleware.RateLimitMiddleware(10, time.Second)

	main := a.Router.Group("/api", rateLimiter)
	{
		// HEAD /api/heartbeat 		-> Used to check if the server is alive
		main.HEAD("/heartbeat", a.Heartbeat)

		// HEAD /api/validate		-> Validates a JWT token
		main.HEAD("/validate", jwt, a.Validate)
	}

	users := main.Group("/users", middleware.BodySizeLimiter(1<<20))
	{
		// GET /api/users		-> Returns the basic info of a user
		users.GET("", jwt, a.UserFetch)

		// POST /api/users 		-> Registers a new user
		users.POST("", a.UserRegister)

		// POST /api/users/login 	-> Logs in a user and returns a JWT token
		users.POST("/login", a.UserLogin)

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
		files.POST("", middleware.BodySizeLimiter(maxUploadSize), a.FileUpload)

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
		ffmpeg.POST("/process", turnstile, middleware.BodySizeLimiter(maxUploadSize), a.FFmpegProcess)
	}

	a.Argon = security.New()
	s3, err := aws.NewS3()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client, %w", err)
	}

	a.S3 = s3
	a.JobQueue.StartWorkerPool()

	return a, nil
}

func makeLogger() {
	atom := zap.NewAtomicLevel()

	switch viper.GetString("app.log_level") {
	case "debug":
		atom.SetLevel(zap.DebugLevel)
	case "warn":
		atom.SetLevel(zap.WarnLevel)
	case "error":
		atom.SetLevel(zap.ErrorLevel)
	case "fatal":
		atom.SetLevel(zap.FatalLevel)
	default:
		atom.SetLevel(zap.InfoLevel)
	}

	cfg := zap.NewDevelopmentConfig()
	cfg.Level = atom
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.EncodeTime = func(t time.Time, pae zapcore.PrimitiveArrayEncoder) {
		pae.AppendString(gray + t.Format("15:04:05.000") + reset)
	}
	cfg.EncoderConfig.EncodeCaller = func(ec zapcore.EntryCaller, pae zapcore.PrimitiveArrayEncoder) {
		pae.AppendString(gray + ec.TrimmedPath() + reset)
	}

	cfg.DisableStacktrace = true

	log, _ := cfg.Build()
	zap.ReplaceGlobals(log)
}

// TODO: use redis instead
func cacheFor(sec int) gin.HandlerFunc {
	return cache.CacheByRequestURI(store, time.Second*time.Duration(sec))
}
