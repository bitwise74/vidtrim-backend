package api

import (
	"bitwise74/video-api/model"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *API) UserLogin(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)

	var data loginBody
	if err := c.ShouldBind(&data); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Invalid request body",
			"requestID": requestID,
		})

		zap.L().Error("Can't bind request body", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	if data.Email == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Email field can't be empty",
			"requestID": requestID,
		})
		return
	}

	if data.Password == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Password field can't be empty",
			"requestID": requestID,
		})
		return
	}

	var user model.User

	if err := a.DB.Where("email = ?", data.Email).First(&user).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error":     "User not found",
			"requestID": requestID,
		})

		zap.L().Error("User not found", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	ok, err := a.Argon.VerifyPasswd(data.Password, user.PasswordHash)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to verify password", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":     "Invalid credentials",
			"requestID": requestID,
		})
		return
	}

	authToken, err := makeToken(&jwt.MapClaims{
		"user_id": user.ID,
		"type":    "auth",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour * 24 * 30).Unix(),
	})
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to generate JWT auth token", zap.Error(err), zap.String("requestID", requestID))
		return
	}

	c.SetCookie("auth_token", authToken, 60*60*24*30, "/", "", viper.GetBool("ssl.enabled"), true)
	c.SetCookie("logged_in", "1", 60*60*24*30, "/", "", viper.GetBool("ssl.enabled"), false)
	c.JSON(http.StatusOK, gin.H{
		"userID": user.ID,
	})
}

func makeToken(c *jwt.MapClaims) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString([]byte(viper.GetString("jwt_secret")))
}
