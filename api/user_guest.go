package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (a *API) UserGuest(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
	// requestID := c.MustGet("requestID").(string)

	// clientIP := c.ClientIP()
	// if clientIP == "" {
	// 	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
	// 		"error":     "Could not determine client IP",
	// 		"requestID": requestID,
	// 	})
	// 	return
	// }

	// var guest model.Guest
	// err := a.DB.
	// 	Where("id = ?", clientIP).
	// 	First(&guest).
	// 	Error
	// if err != nil && err != gorm.ErrRecordNotFound {
	// 	c.JSON(http.StatusInternalServerError, gin.H{
	// 		"error":     "Internal server error",
	// 		"requestID": requestID,
	// 	})
	// 	return
	// } else {

	// }

	// guestID, err := c.Cookie("guest_id")
}
