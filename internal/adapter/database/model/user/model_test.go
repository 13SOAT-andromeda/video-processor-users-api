package user_test

import (
	"testing"
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/model/user"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestToDomain(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	m := &user.Model{
		ID:        id,
		Name:      "Alice",
		Email:     "alice@example.com",
		Document:  "652.904.150-84",
		CreatedAt: now,
		UpdatedAt: now,
	}

	d := m.ToDomain()
	assert.Equal(t, id, d.ID)
	assert.Equal(t, "Alice", d.Name)
	assert.Equal(t, "alice@example.com", d.Email)
	assert.Equal(t, "652.904.150-84", d.Document)
	assert.Equal(t, now, d.CreatedAt)
}

func TestFromDomain(t *testing.T) {
	id := uuid.New()
	d := &domain.User{
		ID:       id,
		Name:     "Bob",
		Email:    "bob@example.com",
		Document: "652.904.150-84",
	}

	m := &user.Model{}
	m.FromDomain(d)
	assert.Equal(t, id, m.ID)
	assert.Equal(t, "Bob", m.Name)
	assert.Equal(t, "bob@example.com", m.Email)
	assert.Equal(t, "652.904.150-84", m.Document)
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
