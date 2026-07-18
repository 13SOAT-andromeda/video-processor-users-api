package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type UpdateUserRequest struct {
	Name     string `json:"name"`
	Document string `json:"document"`
}

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Document  string    `json:"document"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	GetByID(ctx context.Context, id uuid.UUID) (*UserResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateUserRequest) (*UpdateUserResponse, error)
	Delete(ctx context.Context, id uuid.UUID) (*DeleteUserResponse, error)
	List(ctx context.Context, limit, offset int) (*UserListResponse, error)
}
