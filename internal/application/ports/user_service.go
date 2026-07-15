package ports

import (
	"context"
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
)

type CreateUserRequest struct {
	Name     string      `json:"name"     binding:"required"`
	Email    string      `json:"email"    binding:"required"`
	Password string      `json:"password" binding:"required"`
	Role     domain.Role `json:"role"     binding:"required"`
	Document string      `json:"document" binding:"required"`
}

type UpdateUserRequest struct {
	Name     string      `json:"name"`
	Email    string      `json:"email"`
	Password string      `json:"password"`
	Role     domain.Role `json:"role"`
	Document string      `json:"document"`
}

type UserResponse struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Email     string      `json:"email"`
	Role      domain.Role `json:"role"`
	Document  string      `json:"document"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type CreateUserResponse struct {
	Status  string    `json:"status"`
	Message string    `json:"message"`
	ID      uuid.UUID `json:"id"`
}

type UpdateUserResponse struct {
	Status  string    `json:"status"`
	Message string    `json:"message"`
	ID      uuid.UUID `json:"id"`
}

type DeleteUserResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type UserListResponse struct {
	Users  []*UserResponse `json:"users"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type UserService interface {
	Create(ctx context.Context, req CreateUserRequest) (*CreateUserResponse, error)
	GetByID(ctx context.Context, id uuid.UUID) (*UserResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateUserRequest) (*UpdateUserResponse, error)
	Delete(ctx context.Context, id uuid.UUID) (*DeleteUserResponse, error)
	List(ctx context.Context, limit, offset int) (*UserListResponse, error)
}
