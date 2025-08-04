package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type response struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func NewTurnstileMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !viper.GetBool("cloudflare.turnstile.enabled") {
			c.Next()
			return
		}

		token := c.Request.Header.Get("TurnstileToken")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Missing or invalid turnstile token",
			})
			return
		}

		payload := gin.H{
			"secret":   viper.GetString("cloudflare.turnstile.secret_token"),
			"response": token,
			"remoteip": c.ClientIP(),
		}

		jsonBody, _ := json.Marshal(payload)
		resp, err := http.Post("https://challenges.cloudflare.com/turnstile/v0/siteverify", "application/json", bytes.NewReader(jsonBody))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			return
		}

		respBody, _ := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		var res response
		if err := json.Unmarshal(respBody, &res); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized",
			})
			return
		}

		c.Next()
	}
}
