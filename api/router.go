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
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
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

	/* Endpoints table (all start with /api)
	METHOD	ENDPOINT		COMMENT

	--- Health and token validation ---
	HEAD	/heartbeat		Checks if server is alive
	HEAD	/validate		Validates a JWT token

	--- Users ---
	GET	/users			Returns stats of the user invoking request
	POST	/users			Registers a new user
	POST	/users/login		Manages user login
	DELETE	/users/:id		Deletes a user

	--- User settings ---
	GET	/users/me/settings	Returns the current settings of a user
	PATCH	/users/me/settings	Updates user's settings

	--- Admin ---
	GET	/admin/users		Lists registered users
	POST 	/admin/users		Creates a new user
	PATCH	/admin/users/:id	Updates a user
	DELETE  /admin/users/:id	Deletes a user optionally marking as banned

	--- App management ---
	GET	/admin/app/stats	Returns app metrics
	GET	/admin/app/config	Returns current app config
	PATCH	/admin/app/config	Updates the app config

	--- Files ---
	GET 	/files/bulk		Returns user files in bulk
	GET 	/files/search		Searches through uploaded files
	POST	/files			Uploads a file
	DELETE	/files/:id		Deletes a file

	--- FFmpeg ---
	GET	/ffmpeg/start		Initializes an FFmpeg job
	GET	/ffmpeg/progress	Returns job progress
	POST	/ffmpeg/process		Starts an FFmpeg job
	*/

	api := a.Router.Group("/api")

	api.HEAD("/heartbeat", a.Heartbeat)
	api.HEAD("/validate", a.Validate)

	users := api.Group("/users", middleware.BodySizeLimiter(1<<20))
	{
		users.GET("", jwt, a.UserFetch)
		users.POST("", a.UserRegister)
		users.POST("/login", a.UserLogin)
	}

	admin := api.Group("/admin", jwt)
	{
		admin.GET("/users")
		admin.POST("/users")
		admin.PATCH("/users/:id")
		admin.DELETE("/users/:id")

		admin.GET("/app/stats")
		admin.GET("/app/config")
		admin.PATCH("/app/config")
	}

	files := api.Group("/files", jwt)
	{
		files.GET("/bulk", a.FileFetchBulk)
		files.POST("", middleware.BodySizeLimiter(maxUploadSize), a.FileUpload)
		files.DELETE("/:id", a.FileDelete)
		files.GET("/search", cacheFor(15), a.FileSearch)
	}

	ffmpeg := api.Group("/ffmpeg", jwt)
	{
		ffmpeg.GET("/start", a.FFMpegStart)
		ffmpeg.GET("/progress", a.FFmpegProgress)
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

func makeOrigins() []string {
	configOrigins := viper.GetStringSlice("host.cors")

	// lmao
	s := ""
	if viper.GetBool("ssl.enable") {
		s = "s"
	}

	if len(configOrigins) <= 0 {
		return []string{fmt.Sprintf("http%v://%v", s, viper.GetString("host.domain"))}
	}

	origins := make([]string, len(configOrigins))
	for _, v := range configOrigins {
		origins = append(origins, fmt.Sprintf("http%v://%v", s, v))
	}

	origins = append(origins, fmt.Sprintf("http%v://%v", s, viper.GetString("host.domain")))

	return origins
}

// TODO: use redis instead
func cacheFor(sec int) gin.HandlerFunc {
	return cache.CacheByRequestURI(store, time.Second*time.Duration(sec))
}
