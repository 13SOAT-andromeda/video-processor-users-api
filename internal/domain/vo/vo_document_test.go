package vo_test

import (
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain/vo"
	"github.com/stretchr/testify/assert"
)

func TestNewDocument(t *testing.T) {
	t.Run("valid CPF", func(t *testing.T) {
		doc, err := vo.NewDocument("652.904.150-84")
		assert.NoError(t, err)
		assert.Equal(t, vo.Document("652.904.150-84"), doc)
	})

	t.Run("invalid CPF checksum", func(t *testing.T) {
		_, err := vo.NewDocument("123.456.789-00")
		assert.Error(t, err)
	})

	t.Run("all same digits invalid", func(t *testing.T) {
		_, err := vo.NewDocument("111.111.111-11")
		assert.Error(t, err)
	})
}
