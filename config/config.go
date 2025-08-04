// Package config contains code to set the default values and read
// config files to be used throughout the whole application
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	cleanupS3         = pflag.Bool("cleanup-s3", false, "Cleans up S3 bucket")
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
	v.BindEnv("app.log_level", "app_log_level")

	v.BindEnv("host.port", "host_port")
	v.BindEnv("host.domain", "host_domain")

	v.BindEnv("host.ssl.enabled", "host_ssl_enabled")
	v.BindEnv("host.ssl.certificate_path", "host_ssl_certificate_path")
	v.BindEnv("host.ssl.certificate_key_path", "host_ssl_certificate_key_path")

	v.BindEnv("ffmpeg.path", "ffmpeg_path")
	v.BindEnv("ffmpeg.hwaccel_flags", "ffmpeg_hwaccel_flags")

	v.BindEnv("jwt.secret", "jwt_secret")

	v.BindEnv("storage.type", "storage_type")
	v.BindEnv("storage.max_usage", "storage_max_usage")

	v.BindEnv("upload.max_size", "upload_max_size")
	v.BindEnv("upload.allowed_types", "upload_allowed_types")

	v.BindEnv("cloudflare.account_id", "cloudflare_account_id")
	v.BindEnv("cloudflare.access_key_id", "cloudflare_access_key_id")
	v.BindEnv("cloudflare.secret_access_key", "cloudflare_secret_access_key")
	v.BindEnv("cloudflare.bucket", "cloudflare_bucket")

	v.BindEnv("cloudflare.turnstile.enabled", "cloudflare_turnstile_enabled")
	v.BindEnv("cloudflare.turnstile.secret_token", "cloudflare_turnstile_secret_token")

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

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(v.ConfigFileNotFoundError); ok {
			return errors.New("config.toml file is missing")
		}

		return fmt.Errorf("failed to read config file, %w", err)
	}

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
			if v.GetString("cloudflare.account_id") == "" {
				return errors.New("account id can't be empty")
			}
			if v.GetString("cloudflare.access_key_id") == "" {
				return errors.New("account access id can't be empty")
			}
			if v.GetString("cloudflare.secret_access_key") == "" {
				return errors.New("secret access key can't be empty")
			}
			if v.GetString("cloudflare.bucket") == "" {
				return errors.New("bucket can't be empty")
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
