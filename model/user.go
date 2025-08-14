package model

type User struct {
	ID                string `gorm:"primaryKey;autoIncrement" json:"-"`
	PasswordHash      string `gorm:"not null" json:"-"`
	Email             string `gorm:"unique; not null" json:"-"`
	Username          string `json:"username"`
	ProfilePictureKey string `gorm:"default:pfp_placeholder" json:"profile_picture_key"` // S3 key
	Role              string `gorm:"default:user"`
	Language          string `gorm:"default:en"`

	Settings Settings `gorm:"foreignKey:UserID"`
	Files    []File   `gorm:"foreignKey:UserID"`
	Stats    Stats    `gorm:"foreignKey:UserID"`
}
