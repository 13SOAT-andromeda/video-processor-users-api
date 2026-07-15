package services

import (
	"context"
	"errors"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain/vo"
	"github.com/13SOAT-andromeda/video-processor-users-api/pkg/encryption"
	"github.com/google/uuid"
)

type userService struct {
	repo       ports.UserRepository
	authClient ports.AuthServiceClient
	hasher     encryption.Hasher
}

func NewUserService(repo ports.UserRepository, authClient ports.AuthServiceClient, hasher encryption.Hasher) ports.UserService {
	return &userService{repo: repo, authClient: authClient, hasher: hasher}
}

func (s *userService) Create(ctx context.Context, req ports.CreateUserRequest) (*ports.CreateUserResponse, error) {
	if _, err := vo.NewEmail(req.Email); err != nil {
		return nil, err
	}
	if _, err := vo.NewDocument(req.Document); err != nil {
		return nil, err
	}
	if _, err := vo.NewPassword(req.Password); err != nil {
		return nil, err
	}
	if req.Role != domain.RoleAdministrator && req.Role != domain.RoleUser {
		return nil, errors.New("invalid role")
	}

	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, err
	}

	u := &domain.User{
		ID:           uuid.New(),
		Name:         req.Name,
		Email:        req.Email,
		Document:     req.Document,
		Role:         req.Role,
		PasswordHash: hash,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	if err := s.authClient.CreateCredential(ctx, u.Email, u.PasswordHash, u.ID, u.Role); err != nil {
		// rollback lógico
		_ = s.repo.Delete(ctx, u.ID)
		return nil, err
	}

	return &ports.CreateUserResponse{
		Status:  "success",
		Message: "User successfully created",
		ID:      u.ID,
	}, nil
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
	if req.Email != "" {
		if _, err := vo.NewEmail(req.Email); err != nil {
			return nil, err
		}
		u.Email = req.Email
	}
	if req.Document != "" {
		if _, err := vo.NewDocument(req.Document); err != nil {
			return nil, err
		}
		u.Document = req.Document
	}
	if req.Role != "" {
		if req.Role != domain.RoleAdministrator && req.Role != domain.RoleUser {
			return nil, errors.New("invalid role")
		}
		u.Role = req.Role
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
		Role:      u.Role,
		Document:  u.Document,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
