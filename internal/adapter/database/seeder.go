package database

import (
	"context"
	"errors"
	"log"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/model/user"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminSeedConfig struct {
	Email    string
	Password string
	Document string
}

func SeedAdmin(_ context.Context, db *gorm.DB, cfg AdminSeedConfig) error {
	if cfg.Email == "" || cfg.Password == "" || cfg.Document == "" {
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

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	m := &user.Model{
		ID:           uuid.New(),
		Name:         "Administrator",
		Email:        cfg.Email,
		Document:     cfg.Document,
		Role:         string(domain.RoleAdministrator),
		PasswordHash: string(hash),
	}

	if err := db.Create(m).Error; err != nil {
		log.Printf("seeder: admin already exists or error: %v", err)
	}

	return nil
}
