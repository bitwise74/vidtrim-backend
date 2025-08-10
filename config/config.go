// Package config contains code to set the default values and read
// config files to be used throughout the whole application
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"

	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	validLogLevels    = []string{"debug", "info", "warn", "error", "fatal"}
	validStorageTypes = []string{"s3", "local"}
)

func genSecret() string {
	b := make([]byte, 64)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Setup prepares everything config-related so that the app can
// start working. Function will return an error if something
// is critically wrong and the application can't run because of
// that.
func Setup() error {
	pflag.Parse()
	v.BindPFlags(pflag.CommandLine)

	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(".")

	v.AutomaticEnv()

	//
	// ENVS
	//
	v.BindEnv("app.log_level", "APP_LOG_LEVEL")

	v.BindEnv("host.port", "HOST_PORT")
	v.BindEnv("host.domain", "HOST_DOMAIN")
	v.BindEnv("host.cors", "HOST_CORS")

	v.BindEnv("host.ssl.enabled", "HOST_SSL_ENABLED")
	v.BindEnv("host.ssl.certificate_path", "HOST_SSL_CERTIFICATE_PATH")
	v.BindEnv("host.ssl.certificate_key_path", "HOST_SSL_CERTIFICATE_KEY_PATH")

	v.BindEnv("ffmpeg.path", "FFMPEG_PATH")
	v.BindEnv("ffmpeg.hwaccel_flags", "FFMPEG_HWACCEL_FLAGS")
	v.BindEnv("ffmpeg.max_jobs", "FFMPEG_MAX_JOBS")
	v.BindEnv("ffmpeg.workers", "FFMPEG_WORKERS")

	v.BindEnv("jwt.secret", "JWT_SECRET")

	v.BindEnv("storage.type", "STORAGE_TYPE")
	v.BindEnv("storage.max_usage", "STORAGE_MAX_USAGE")

	v.BindEnv("upload.max_size", "UPLOAD_MAX_SIZE")
	v.BindEnv("upload.allowed_types", "UPLOAD_ALLOWED_TYPES")

	v.BindEnv("aws.access_key_id", "AWS_ACCESS_KEY_ID")
	v.BindEnv("aws.secret_access_key", "AWS_SECRET_ACCESS_KEY")
	v.BindEnv("aws.bucket", "AWS_BUCKET")
	v.BindEnv("aws.region", "AWS_REGION")
	v.BindEnv("aws.cloudfront_url", "AWS_CLOUDFRONT_URL")

	v.BindEnv("cloudflare.turnstile.enabled", "CLOUDFLARE_TURNSTILE_ENABLED")
	v.BindEnv("cloudflare.turnstile.secret_token", "CLOUDFLARE_TURNSTILE_SECRET_TOKEN")

	//
	// Defaults
	//
	v.SetDefault("app.log_level", "info")

	v.SetDefault("host.port", 8080)
	v.SetDefault("host.domain", "localhost")

	v.SetDefault("host.ssl_enabled", false)

	v.SetDefault("storage.type", "local")

	v.SetDefault("upload.max_size", 50)
	v.SetDefault("upload.allowed_types", []string{"video/mp4"})

	v.SetDefault("cloudflare.turnstile.enabled", false)

	// Wont do anything for docker
	v.ReadInConfig()

	if !slices.Contains(validLogLevels, v.GetString("app.log_level")) {
		return errors.New("invalid log level provided")
	}

	if v.GetInt("upload.max_size") <= 0 {
		return errors.New("upload.max_size must be bigger than 0")
	}

	if v.GetInt("host.port") <= 0 {
		return errors.New("invalid port provided")
	}

	if v.GetBool("host.ssl.enabled") {
		if v.GetString("host.ssl.certificate_path") == "" {
			return errors.New("no ssl certificate path provided")
		}

		if v.GetString("host.ssl.certificate_key_path") == "" {
			return errors.New("no ssl certificate key path provided")
		}
	}

	// Test if ffmpeg present in path. If not try to use the user provided one
	err := exec.Command("ffmpeg", "-version").Run()
	if err != nil {
		err = exec.Command(v.GetString("ffmpeg.path"), "-version").Run()
		if err != nil {
			return errors.New("ffmpeg not found")
		}
	}

	if v.GetInt("ffmpeg.max_jobs") <= 0 {
		return errors.New("max job queue size must be at least 1")
	}

	if v.GetInt("ffmpeg.workers") <= 0 {
		return errors.New("ffmpeg workers must be set to at least 1")
	}

	if v.GetString("jwt.secret") == "" {
		fmt.Println("WARNING: You haven't set a JWT secret, so it has been generated for you. Please set it as an environment variable or in the config.toml file.\nYour random JWT secret:\n\n" + genSecret() + "\n\nPaste it into your config.toml file.")
		os.Exit(0)
	}

	if v.GetString("upload.allowed_types") == "" {
		zap.L().Warn("No upload.allowed_types specified, any file type will be accepted")
	}

	switch v.GetString("storage.type") {
	case "s3":
		{
			if v.GetString("aws.access_key_id") == "" {
				return errors.New("account access id can't be empty")
			}
			if v.GetString("aws.secret_access_key") == "" {
				return errors.New("secret access key can't be empty")
			}
			if v.GetString("aws.bucket") == "" {
				return errors.New("bucket can't be empty")
			}
			if v.GetString("aws.region") == "" {
				return errors.New("region can't be empty")
			}
			if v.GetString("aws.cloudfront_url") == "" {
				return errors.New("cloudfront url can't be empty")
			}
		}
	case "local":
		{
			return errors.New("not supported yet")
		}
	default:
		return errors.New("invalid storage type provided")
	}

	if !slices.Contains(validStorageTypes, v.GetString("storage.type")) {
		return errors.New("invalid storage type provided")
	}

	if v.GetInt("storage.max_usage") <= 0 {
		return errors.New("max usage must be bigger than 0")
	}

	if v.GetInt("upload.max_size") <= 0 {
		return errors.New("max upload size must be bigger than 0")
	}

	if !v.GetBool("cloudflare.turnstile.enabled") {
		fmt.Println("[WARNING]: Cloudflare's turnstile is disabled. Some public endpoints won't be guarded against bots")
	} else {
		if v.GetString("cloudflare.turnstile.secret_token") == "" {
			return errors.New("turnstile secret token is missing")
		}
	}

	v.Set("upload.max_size", v.GetInt64("upload.max_size")<<20)
	return nil
}
