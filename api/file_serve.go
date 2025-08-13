package api

// This seems to be useless

// type partialData struct {
// 	S3Key  string
// 	Format string
// }

// // FileServe serves a file for viewing on a webiste or in a browser directly from the CDN
// func (a *API) FileServe(c *gin.Context) {
// 	requestID := c.MustGet("requestID").(string)
// 	fileID := c.Param("fileID")
// 	if fileID == "" {
// 		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
// 			"error":     "No file ID provided",
// 			"requestID": requestID,
// 		})
// 		return
// 	}

// 	thumbStr := c.DefaultQuery("t", "1")
// 	thumb, err := strconv.ParseBool(thumbStr)
// 	if err != nil {
// 		thumb = true
// 	}

// 	var data partialData

// 	err = a.DB.
// 		Model(model.File{}).
// 		Where("id = ?", fileID).
// 		Select("s3_key, format").
// 		First(&data).
// 		Error
// 	if err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
// 				"error":     "File not found",
// 				"requestID": requestID,
// 			})
// 			return
// 		}

// 		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
// 			"error":     "Internal server error",
// 			"requestID": requestID,
// 		})

// 		zap.L().Error("Failed to check if file exists", zap.String("id", fileID), zap.Error(err))
// 		return
// 	}

// 	if thumb {
// 		data.S3Key = "thumb_" + data.S3Key
// 	}

// 	c.Redirect(http.StatusFound, viper.GetString("aws.cloudfront_url")+"/"+data.S3Key)
// }
