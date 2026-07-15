package ports

import (
	"context"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
)

type AuthServiceClient interface {
	CreateCredential(ctx context.Context, email, passwordHash string, userID uuid.UUID, role domain.Role) error
}
