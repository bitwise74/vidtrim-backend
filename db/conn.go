// Package db contains things related to SQlite
package db

import (
	"bitwise74/video-api/model"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func New() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("database.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SQLite database, %w", err)
	}

	err = db.AutoMigrate(model.User{}, model.File{}, model.Stats{}, model.VerificationToken{}, model.ResendRequest{}, model.Migration{})
	if err != nil {
		return nil, fmt.Errorf("failed to automigrate tables, %w", err)
	}

	return db, nil
}
