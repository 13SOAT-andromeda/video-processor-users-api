package user_test

import (
	"testing"
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/model/user"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestToDomain(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	m := &user.Model{
		ID:           id,
		Name:         "Alice",
		Email:        "alice@example.com",
		Document:     "652.904.150-84",
		Role:         "administrator",
		PasswordHash: "hash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	d := m.ToDomain()
	assert.Equal(t, id, d.ID)
	assert.Equal(t, "Alice", d.Name)
	assert.Equal(t, "alice@example.com", d.Email)
	assert.Equal(t, domain.RoleAdministrator, d.Role)
	assert.Nil(t, d.DeletedAt)
}

func TestToDomain_WithDeletedAt(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	m := &user.Model{
		ID:        id,
		DeletedAt: gorm.DeletedAt{Time: now, Valid: true},
	}
	d := m.ToDomain()
	assert.NotNil(t, d.DeletedAt)
}

func TestFromDomain(t *testing.T) {
	id := uuid.New()
	d := &domain.User{
		ID:           id,
		Name:         "Bob",
		Email:        "bob@example.com",
		Document:     "652.904.150-84",
		Role:         domain.RoleUser,
		PasswordHash: "hash123",
	}

	m := &user.Model{}
	m.FromDomain(d)
	assert.Equal(t, id, m.ID)
	assert.Equal(t, "Bob", m.Name)
	assert.Equal(t, "user", m.Role)
	assert.Equal(t, "hash123", m.PasswordHash)
}

func TestFromDomain_Nil(t *testing.T) {
	m := &user.Model{}
	m.FromDomain(nil)
	assert.Equal(t, uuid.Nil, m.ID)
}

func TestTableName(t *testing.T) {
	m := &user.Model{}
	assert.Equal(t, "users", m.TableName())
}
