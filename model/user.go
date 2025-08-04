package model

type User struct {
	ID           string `gorm:"primaryKey;autoIncrement"`
	Email        string `gorm:"unique; not null"`
	PasswordHash string `gorm:"not null"`

	Files []File `gorm:"foreignKey:UserID"`
	Stats Stats  `gorm:"foreignKey:UserID"`
}
