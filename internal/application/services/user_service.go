package services

import (
	"context"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
)

type userService struct {
	repo ports.UserRepository
}

func NewUserService(repo ports.UserRepository) ports.UserService {
	return &userService{repo: repo}
}

func (s *userService) GetByID(ctx context.Context, id uuid.UUID) (*ports.UserResponse, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toUserResponse(u), nil
}

func (s *userService) Update(ctx context.Context, id uuid.UUID, req ports.UpdateUserRequest) (*ports.UpdateUserResponse, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		u.Name = req.Name
	}
	if req.Document != "" {
		u.Document = req.Document
	}

	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}

	return &ports.UpdateUserResponse{
		Status:  "success",
		Message: "User successfully updated",
		ID:      u.ID,
	}, nil
}

func (s *userService) Delete(ctx context.Context, id uuid.UUID) (*ports.DeleteUserResponse, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	return &ports.DeleteUserResponse{
		Status:  "success",
		Message: "User successfully deleted",
	}, nil
}

func (s *userService) List(ctx context.Context, limit, offset int) (*ports.UserListResponse, error) {
	users, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	resp := make([]*ports.UserResponse, len(users))
	for i, u := range users {
		resp[i] = toUserResponse(u)
	}
	return &ports.UserListResponse{Users: resp, Limit: limit, Offset: offset}, nil
}

func toUserResponse(u *domain.User) *ports.UserResponse {
	return &ports.UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Document:  u.Document,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
