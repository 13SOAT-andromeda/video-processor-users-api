package encryption_test

import (
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/pkg/encryption"
	"github.com/stretchr/testify/assert"
)

func TestBcryptHasher(t *testing.T) {
	h := encryption.NewBcryptHasher()

	hash, err := h.Hash("mypassword")
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "mypassword", hash)

	assert.True(t, h.Compare(hash, "mypassword"))
	assert.False(t, h.Compare(hash, "wrong"))
}
