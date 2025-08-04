package model

type Guest struct {
	ID               string `gorm:"primaryKey;type:uuid"`
	IP               string `gorm:"primaryKey;index"`
	UserAgent        string `gorm:"size:512"`
	SessionCreatedAt int64
	SessionExpiresAt int64
	LastActiveAt     int64
	FFmpegJobs       int
	IsBlocked        bool
}
