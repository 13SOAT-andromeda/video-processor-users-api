package user

import (
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Model struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name         string         `gorm:"not null"`
	Email        string         `gorm:"uniqueIndex;not null"`
	Document     string         `gorm:"not null"`
	Role         string         `gorm:"not null"`
	PasswordHash string         `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (*Model) TableName() string {
	return "users"
}

func (m *Model) ToDomain() *domain.User {
	var deletedAt *time.Time
	if m.DeletedAt.Valid {
		deletedAt = &m.DeletedAt.Time
	}
	return &domain.User{
		ID:           m.ID,
		Name:         m.Name,
		Email:        m.Email,
		Document:     m.Document,
		Role:         domain.Role(m.Role),
		PasswordHash: m.PasswordHash,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		DeletedAt:    deletedAt,
	}
}

func (m *Model) FromDomain(d *domain.User) {
	if d == nil {
		return
	}
	m.ID = d.ID
	m.Name = d.Name
	m.Email = d.Email
	m.Document = d.Document
	m.Role = string(d.Role)
	m.PasswordHash = d.PasswordHash
}
