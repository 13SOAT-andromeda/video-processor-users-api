package domain_test

import (
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, domain.Role("administrator"), domain.RoleAdministrator)
	assert.Equal(t, domain.Role("user"), domain.RoleUser)
}
