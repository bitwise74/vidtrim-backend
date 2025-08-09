package api

import (
	"bitwise74/video-api/model"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var validLimits = []int{10, 20, 50, 100, 250}

func (a *API) FileSearch(c *gin.Context) {
	requestID := c.MustGet("requestID").(string)
	userID := c.MustGet("userID").(string)

	searchQuery := strings.ToLower(c.Query("query"))
	if searchQuery == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "No search query provided",
			"requestID": requestID,
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || !slices.Contains(validLimits, limit) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Invalid limit provided",
			"requestID": requestID,
		})
		return
	}

	pageStr := c.DefaultQuery("limit", "0")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":     "Invalid page provided",
			"requestID": requestID,
		})
		return
	}

	var results []model.File

	err = a.DB.
		Where("user_id = ? AND original_name NOT LIKE ? AND LOWER(original_name) LIKE ? ", userID, "thumb_%", "%"+searchQuery+"%").
		Order("created_at desc").
		Offset(page * limit).
		Limit(limit).
		Find(&results).
		Error
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":     "Internal server error",
			"requestID": requestID,
		})

		zap.L().Error("Failed to find files by search query", zap.Error(err))
		return
	}

	c.JSON(http.StatusOK, results)
}
