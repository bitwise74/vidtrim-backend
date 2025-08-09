// Package model defines database models
package model

type File struct {
	ID     uint   `gorm:"primaryKey;autoIncrement;index" json:"id"`
	UserID string `json:"-"`

	// Since we want to allow different users to have files with the same name we
	// need to keep the S3 objects under a different key
	S3Key string `json:"s3_key"`

	// Original file name before turning it into a special S3 key
	OriginalName string `json:"name"`
	// At first it seems like storing both the URL and the thumbnail URL
	// makes sense since these could be easily shared when a user accesses a file
	// but in reality its better to create the URL on the go when a file is requested
	// because the URL just includes the name of the file and the thumbnail URL is the
	// same shit just prefixed with thumbnail_
	// URL          string `json:"url"`
	// ThumbnailURL string `json:"thumbnailURL"`
	Private bool   `json:"private"`
	Format  string `json:"format"`
	Views   int32  `json:"views"`
	Size    int64  `json:"size"`
	// Needed for correct stat calculations but not shown to the user. Thumbnails aren't taken into
	// account for files to not confuse people on the space used being "invalid"
	ThumbnailSize int64 `json:"-"`
	// Used to inform the frontend/backend if the file is being processed/uploaded
	State string `json:"state"`
	// All are unix millisecond timestamps
	Duration  float64 `json:"duration"`
	CreatedAt int64   `gorm:"not null" json:"created_at"`
	ExpiresAt *int64  `json:"expires_at,omitzero"`
}
