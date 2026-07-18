package user

import (
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
)

type Model struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"not null"`
	Email     string    `gorm:"uniqueIndex;not null"`
	Document  string    `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (*Model) TableName() string {
	return "users"
}

func (m *Model) ToDomain() *domain.User {
	return &domain.User{
		ID:        m.ID,
		Name:      m.Name,
		Email:     m.Email,
		Document:  m.Document,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
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
}
