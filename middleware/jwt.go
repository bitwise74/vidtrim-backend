package middleware

import (
	"bitwise74/video-api/model"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func NewJWTMiddleware(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.MustGet("requestID").(string)

		tokenStr, err := c.Cookie("auth_token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
			}

			return []byte(viper.GetString("jwt_secret")), nil
		})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":     "Invalid token",
				"requestID": requestID,
			})

			zap.L().Error("Failed to parse token", zap.Error(err), zap.String("requestID", requestID))
			return
		}

		if !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Missing or invalid token claims",
				"requestID": requestID,
			})
			return
		}

		userID, ok := claims["user_id"].(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})
			return
		}

		expRaw, ok := claims["exp"]
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":     "Token expired or invalid format",
				"requestID": requestID,
			})
			return
		}

		exp, ok := expRaw.(float64)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})
			return
		}

		if time.Now().Unix() >= int64(exp) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":     "Token expired",
				"requestID": requestID,
			})
			return
		}

		// In case someone logs in to delete their account and then logs in again (that's so fucking stupid I know),
		// we'll reject the request
		var found bool
		err = d.Model(model.User{}).Where("id = ?", userID).Select("count(*) > 0").First(&found).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":     "Internal server error",
				"requestID": requestID,
			})

			zap.L().Error("Failed to check if user exists", zap.Error(err), zap.String("requestID", requestID))
			return
		}

		if !found {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error":     "User not found",
				"requestID": requestID,
			})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}
