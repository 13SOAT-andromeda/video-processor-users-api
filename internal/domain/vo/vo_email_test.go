package vo_test

import (
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain/vo"
	"github.com/stretchr/testify/assert"
)

func TestNewEmail(t *testing.T) {
	t.Run("valid email", func(t *testing.T) {
		email, err := vo.NewEmail("user@example.com")
		assert.NoError(t, err)
		assert.Equal(t, vo.Email("user@example.com"), email)
	})

	t.Run("invalid email", func(t *testing.T) {
		_, err := vo.NewEmail("not-an-email")
		assert.Error(t, err)
	})
}
