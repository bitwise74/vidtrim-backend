package api

import (
	"bitwise74/video-api/model"
	"bitwise74/video-api/validators"
	"net/http"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/spf13/viper"
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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Can't bind request body", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	if err := validators.EmailValidator(data.Email); err != nil {
		zap.L().Debug("Invalid email", zap.Error(err), zap.String("requestID", requestID))

		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     err.Error(),
			"requestID": requestID,
		})
		return
	}

	if err := validators.PasswordValidator(data.Password); err != nil {
		zap.L().Debug("Invalid password", zap.Error(err), zap.String("requestID", requestID))

		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
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
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to check if user is registered", zap.Error(r.Error), zap.String("requestID", requestID))
		return
	}

	if found {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error":     "This email is already registered. Please login or use a different email",
			"requestID": requestID,
		})
		return
	}

	hash, err := a.Argon.GenerateFromPassword(data.Password)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to hash password", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	userID, err := gonanoid.Generate(charset, 16)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to generate user ID", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	if err := a.DB.Create(&model.User{
		ID:           userID,
		Email:        data.Email,
		PasswordHash: hash,
		Stats: model.Stats{
			UserID:     userID,
			MaxStorage: viper.GetInt64("storage.max_usage"),
		},
	}).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to create user", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	c.Status(http.StatusOK)
}
