// Package api contains all endpoints available
package api

import (
	"bitwise74/video-api/cloudflare"
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
	R2       *cloudflare.R2Client
	JobQueue *service.JobQueue
}

func NewRouter() (*API, error) {
	a := &API{
		JobQueue: service.NewJobQueue(),
	}

	db, err := db.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SQLite database, %w", err)
	}
	a.DB = db

	makeLogger()

	router := gin.New()
	a.Router = router

	router.Use(
		cors.New(cors.Config{
			AllowOrigins:     []string{"http://localhost:5173"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "TurnstileToken"},
			ExposeHeaders:    []string{"Content-Length"},
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

	router.HandleMethodNotAllowed = true
	router.RedirectFixedPath = true
	a.Router.MaxMultipartMemory = 5 << 20

	jwt := middleware.NewJWTMiddleware(db)
	turnstile := middleware.NewTurnstileMiddleware()
	maxUploadSize := viper.GetInt64("upload.max_size")

	main := router.Group("/api")
	{
		// HEAD /api/heartbeat 		-> Used to check if the server is alive
		main.HEAD("/heartbeat", a.Heartbeat)

		// HEAD /api/validate		-> Validates a JWT token
		main.HEAD("/validate", jwt, a.Validate)
	}

	users := main.Group("/users", middleware.BodySizeLimiter(1<<20))
	{
		// GET /api/users		-> Returns the stats of a user
		users.GET("", jwt, cacheFor(30), a.UserFetch)

		// POST /api/users 		-> Registers a new user
		users.POST("", a.UserRegister)

		// POST /api/users/login 	-> Logs in a user and returns a JWT token
		users.POST("/login", cacheFor(30), a.UserLogin)

		// DELETE /api/users/:id 	-> Deletes a user by their ID
		// users.DELETE("/:id", jwt)
	}

	files := main.Group("/files")
	{
		// GET /api/files/:name 	-> Serves a file directly
		files.GET("/:fileID", a.FileServe)

		// GET /api/files/bulk 		-> Returns a user's files in bulk
		files.GET("/bulk", jwt, a.FileFetchBulk)

		// POST /api/files         	-> Uploads a new file and stores it in the database
		files.POST("", jwt, middleware.BodySizeLimiter(maxUploadSize), a.FileUpload)

		// DELETE /api/files/:id	-> Deletes a file owned by a user
		files.DELETE("/:id", jwt, a.FileDelete)
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
	s3, err := cloudflare.NewR2()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client, %w", err)
	}

	a.R2 = s3
	a.JobQueue.StartWorkerPool()

	return a, nil
}

func makeLogger() {
	cfg := zap.NewDevelopmentConfig()
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

func cacheFor(sec int) gin.HandlerFunc {
	return cache.CacheByRequestURI(store, time.Second*time.Duration(sec))
}
