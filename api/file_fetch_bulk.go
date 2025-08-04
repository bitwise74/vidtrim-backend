package api

import (
	"bitwise74/video-api/model"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AZ = A - Z as in alphabetic same for ZA
var validSortOpts = []string{"newest", "oldest", "az", "za", "size-asc", "size-desc"}

func (a *API) FileFetchBulk(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	pageStr := c.DefaultQuery("page", "0")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Page is not a valid integer",
			"requestID": requestID,
		})
		return
	}

	if page < 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Page can't be negative",
			"requestID": requestID,
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Limit is not a valid integer",
			"requestID": requestID,
		})
		return
	}

	if limit <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Limit must be bigger than 0",
			"requestID": requestID,
		})
		return
	}

	if limit > 100 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Limit can't be bigger than 100",
			"requestID": requestID,
		})
		return
	}

	sort := strings.ToLower(c.DefaultQuery("sort", "newest"))
	if !slices.Contains(validSortOpts, sort) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Invalid sorting option",
			"requestID": requestID,
		})
		return
	}

	order := ""

	switch sort {
	case "newest":
		order = "created_at desc"
	case "oldest":
		order = "created_at asc"
	case "az":
		order = "name"
	case "za":
		order = "name desc"
	case "size-asc":
		order = "size asc"
	case "size-desc":
		order = "size desc"
	}

	offset := page * limit
	var entries []model.File

	err = a.DB.
		Where("user_id = ?", userID).
		Order(order).
		Offset(offset).
		Limit(limit).
		Find(&entries).
		Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error":     "No files found",
				"requestID": requestID,
			})
			return
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})
		zap.L().Error("Failed to lookup user files", zap.Error(err))
		return
	}

	c.JSON(http.StatusOK, entries)
}
