package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type client struct {
	requests int
	lastSeen time.Time
}

var clients = make(map[string]*client)
var mu sync.Mutex

func RateLimitMiddleware(maxRequests int, duration time.Duration) gin.HandlerFunc {
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > duration {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		cl, exists := clients[ip]
		if !exists {
			cl = &client{requests: 1, lastSeen: time.Now()}
			clients[ip] = cl
		} else {
			cl.requests++
			cl.lastSeen = time.Now()
		}
		mu.Unlock()

		if cl.requests > maxRequests {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
