package api

import (
	"bitwise74/video-api/model"
	"bitwise74/video-api/security"
	"bitwise74/video-api/service"
	"bitwise74/video-api/validators"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type registerBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *API) UserRegister(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)

	var data registerBody
	if err := c.ShouldBind(&data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Can't bind request body", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	if err := validators.EmailValidator(data.Email); err != nil {
		zap.L().Debug("Invalid email", zap.Error(err), zap.String("requestID", requestID))

		c.JSON(http.StatusBadRequest, gin.H{
			"error":     err.Error(),
			"requestID": requestID,
		})
		return
	}

	if err := validators.PasswordValidator(data.Password); err != nil {
		zap.L().Debug("Invalid password", zap.Error(err), zap.String("requestID", requestID))

		c.JSON(http.StatusBadRequest, gin.H{
			"error":     err.Error(),
			"requestID": requestID,
		})
		return
	}

	var found bool

	r := a.DB.Model(model.User{}).
		Select("count(*) > 0").
		Where("email = ?", data.Email).
		First(&found)
	if r.Error != nil && r.Error != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to check if user is registered", zap.Error(r.Error), zap.String("requestID", requestID))
		return
	}

	if found {
		c.JSON(http.StatusConflict, gin.H{
			"error":     "This email is already registered. Please login or use a different email",
			"requestID": requestID,
		})
		return
	}

	hash, err := a.Argon.GenerateFromPassword(data.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to hash password", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	userID, err := gonanoid.Generate(charset, 16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to generate user ID", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	expireAt := time.Now().Add(time.Minute * 30)
	cleanAt := time.Now().Add(time.Hour * 24 * 60)

	verifToken, err := security.MakeVerificationToken(&security.VerificationTokenOpts{
		UserID:    userID,
		Purpose:   "email_verify",
		ExpiresAt: &expireAt, // Expire after 30 minutes
		CleanupAt: &cleanAt,  // Cleanup after 60 days
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to generate verification token", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	// Try to send mail now
	err = service.SendVerificationMail(verifToken, data.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to send verification email", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	maxStorage, _ := strconv.ParseInt(os.Getenv("STORAGE_MAX_USAGE"), 10, 64)

	if err := a.DB.Create(&model.User{
		ID:           userID,
		Email:        data.Email,
		PasswordHash: hash,
		Stats: model.Stats{
			UserID:     userID,
			MaxStorage: maxStorage,
		},
		VerificationTokens: []model.VerificationToken{
			*verifToken,
		},
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to create user", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	sslEnabled, err := strconv.ParseBool("HOST_SSL_ENABLED")
	if err != nil {
		sslEnabled = false
	}

	c.SetCookie("user_id", userID, 9999999, "/", "", sslEnabled, false)

	c.JSON(http.StatusOK, gin.H{
		"userID": userID,
	})
}
