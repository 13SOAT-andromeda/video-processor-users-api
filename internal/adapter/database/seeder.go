package database

import (
	"context"
	"errors"
	"log"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/model/user"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AdminSeedConfig struct {
	Email    string
	Document string
	Name     string
}

func SeedAdmin(_ context.Context, db *gorm.DB, cfg AdminSeedConfig) error {
	if cfg.Email == "" || cfg.Document == "" {
		return nil
	}

	var existing user.Model
	err := db.Where("email = ?", cfg.Email).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	name := cfg.Name
	if name == "" {
		name = "Administrator"
	}

	m := &user.Model{
		ID:       uuid.New(),
		Name:     name,
		Email:    cfg.Email,
		Document: cfg.Document,
	}

	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoNothing: true,
	}).Create(m).Error
	if err != nil {
		log.Printf("seeder: %v", err)
	}

	return nil
}
