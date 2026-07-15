package domain_test

import (
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestValidRole(t *testing.T) {
	u := &domain.User{Role: domain.RoleAdministrator}
	assert.True(t, u.ValidRole())

	u.Role = domain.RoleUser
	assert.True(t, u.ValidRole())

	u.Role = "unknown"
	assert.False(t, u.ValidRole())
}
