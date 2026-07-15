package services_test

import (
	"context"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// --- UserRepository mock ---

type mockUserRepository struct{ mock.Mock }

func (m *mockUserRepository) Create(ctx context.Context, u *domain.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *mockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *mockUserRepository) Update(ctx context.Context, u *domain.User) error {
	args := m.Called(ctx, u)
	return args.Error(0)
}

func (m *mockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUserRepository) List(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

// --- AuthServiceClient mock ---

type mockAuthClient struct{ mock.Mock }

func (m *mockAuthClient) CreateCredential(ctx context.Context, email, passwordHash string, userID uuid.UUID, role domain.Role) error {
	args := m.Called(ctx, email, passwordHash, userID, role)
	return args.Error(0)
}

// --- Hasher mock ---

type mockHasher struct{ mock.Mock }

func (m *mockHasher) Hash(plain string) (string, error) {
	args := m.Called(plain)
	return args.String(0), args.Error(1)
}

func (m *mockHasher) Compare(hash, plain string) bool {
	args := m.Called(hash, plain)
	return args.Bool(0)
}
