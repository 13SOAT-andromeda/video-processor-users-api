package vo_test

import (
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain/vo"
	"github.com/stretchr/testify/assert"
)

func TestNewPassword(t *testing.T) {
	t.Run("valid password", func(t *testing.T) {
		pass, err := vo.NewPassword("Admin@123456")
		assert.NoError(t, err)
		assert.Equal(t, vo.Password("Admin@123456"), pass)
	})

	t.Run("too short", func(t *testing.T) {
		_, err := vo.NewPassword("Ab1!")
		assert.Error(t, err)
	})

	t.Run("no uppercase", func(t *testing.T) {
		_, err := vo.NewPassword("password1!")
		assert.Error(t, err)
	})

	t.Run("no number", func(t *testing.T) {
		_, err := vo.NewPassword("Password!")
		assert.Error(t, err)
	})

	t.Run("no symbol", func(t *testing.T) {
		_, err := vo.NewPassword("Password1")
		assert.Error(t, err)
	})
}
