package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/services"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreate_Success(t *testing.T) {
	repo := &mockUserRepository{}
	auth := &mockAuthClient{}
	hasher := &mockHasher{}
	svc := services.NewUserService(repo, auth, hasher)

	hasher.On("Hash", mock.AnythingOfType("string")).Return("hashed", nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)
	auth.On("CreateCredential", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	req := ports.CreateUserRequest{
		Name:     "John Doe",
		Email:    "john@example.com",
		Password: "Admin@123456",
		Role:     domain.RoleAdministrator,
		Document: "652.904.150-84",
	}

	resp, err := svc.Create(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestCreate_EmailAlreadyExists(t *testing.T) {
	repo := &mockUserRepository{}
	auth := &mockAuthClient{}
	hasher := &mockHasher{}
	svc := services.NewUserService(repo, auth, hasher)

	hasher.On("Hash", mock.Anything).Return("hashed", nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(domain.ErrEmailAlreadyExists)

	req := ports.CreateUserRequest{
		Name: "Jane", Email: "jane@example.com",
		Password: "Admin@123456", Role: domain.RoleUser, Document: "652.904.150-84",
	}

	_, err := svc.Create(context.Background(), req)
	assert.ErrorIs(t, err, domain.ErrEmailAlreadyExists)
}

func TestCreate_AuthClientFails_Rollback(t *testing.T) {
	repo := &mockUserRepository{}
	auth := &mockAuthClient{}
	hasher := &mockHasher{}
	svc := services.NewUserService(repo, auth, hasher)

	hasher.On("Hash", mock.Anything).Return("hashed", nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)
	auth.On("CreateCredential", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("auth-api down"))
	repo.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)

	req := ports.CreateUserRequest{
		Name: "Bob", Email: "bob@example.com",
		Password: "Admin@123456", Role: domain.RoleUser, Document: "652.904.150-84",
	}

	_, err := svc.Create(context.Background(), req)
	assert.Error(t, err)
	repo.AssertCalled(t, "Delete", mock.Anything, mock.AnythingOfType("uuid.UUID"))
}

func TestGetByID_NotFound(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo, &mockAuthClient{}, &mockHasher{})

	id := uuid.New()
	repo.On("FindByID", mock.Anything, id).Return(nil, domain.ErrUserNotFound)

	_, err := svc.GetByID(context.Background(), id)
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestGetByID_Success(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo, &mockAuthClient{}, &mockHasher{})

	id := uuid.New()
	u := &domain.User{ID: id, Name: "Alice", Email: "alice@example.com", Role: domain.RoleUser, Document: "652.904.150-84", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)

	resp, err := svc.GetByID(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, id, resp.ID)
	assert.Equal(t, "Alice", resp.Name)
}

func TestDelete_Success(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo, &mockAuthClient{}, &mockHasher{})

	id := uuid.New()
	u := &domain.User{ID: id}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)
	repo.On("Delete", mock.Anything, id).Return(nil)

	resp, err := svc.Delete(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestList_Success(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo, &mockAuthClient{}, &mockHasher{})

	users := []*domain.User{
		{ID: uuid.New(), Name: "User1", Email: "u1@x.com", Role: domain.RoleUser, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	repo.On("List", mock.Anything, 10, 0).Return(users, nil)

	resp, err := svc.List(context.Background(), 10, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Users, 1)
}

func TestUpdate_Success(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo, &mockAuthClient{}, &mockHasher{})

	id := uuid.New()
	u := &domain.User{ID: id, Name: "Old", Email: "old@x.com", Role: domain.RoleUser, Document: "652.904.150-84"}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	req := ports.UpdateUserRequest{Name: "New", Email: "new@x.com", Document: "652.904.150-84", Role: domain.RoleAdministrator}
	resp, err := svc.Update(context.Background(), id, req)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestUpdate_InvalidEmail(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo, &mockAuthClient{}, &mockHasher{})

	id := uuid.New()
	u := &domain.User{ID: id}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)

	req := ports.UpdateUserRequest{Email: "not-an-email"}
	_, err := svc.Update(context.Background(), id, req)
	assert.Error(t, err)
}

func TestUpdate_InvalidDocument(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo, &mockAuthClient{}, &mockHasher{})

	id := uuid.New()
	u := &domain.User{ID: id}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)

	req := ports.UpdateUserRequest{Document: "000.000.000-00"}
	_, err := svc.Update(context.Background(), id, req)
	assert.Error(t, err)
}

func TestUpdate_InvalidRole(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo, &mockAuthClient{}, &mockHasher{})

	id := uuid.New()
	u := &domain.User{ID: id}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)

	req := ports.UpdateUserRequest{Role: "superuser"}
	_, err := svc.Update(context.Background(), id, req)
	assert.Error(t, err)
}

func TestCreate_InvalidEmail(t *testing.T) {
	svc := services.NewUserService(&mockUserRepository{}, &mockAuthClient{}, &mockHasher{})
	_, err := svc.Create(context.Background(), ports.CreateUserRequest{
		Name: "X", Email: "bad", Password: "Admin@1!", Role: domain.RoleUser, Document: "652.904.150-84",
	})
	assert.Error(t, err)
}

func TestCreate_InvalidDocument(t *testing.T) {
	svc := services.NewUserService(&mockUserRepository{}, &mockAuthClient{}, &mockHasher{})
	_, err := svc.Create(context.Background(), ports.CreateUserRequest{
		Name: "X", Email: "x@x.com", Password: "Admin@1!", Role: domain.RoleUser, Document: "000.000.000-00",
	})
	assert.Error(t, err)
}

func TestCreate_InvalidPassword(t *testing.T) {
	svc := services.NewUserService(&mockUserRepository{}, &mockAuthClient{}, &mockHasher{})
	_, err := svc.Create(context.Background(), ports.CreateUserRequest{
		Name: "X", Email: "x@x.com", Password: "weak", Role: domain.RoleUser, Document: "652.904.150-84",
	})
	assert.Error(t, err)
}

func TestCreate_InvalidRole(t *testing.T) {
	svc := services.NewUserService(&mockUserRepository{}, &mockAuthClient{}, &mockHasher{})
	_, err := svc.Create(context.Background(), ports.CreateUserRequest{
		Name: "X", Email: "x@x.com", Password: "Admin@123!", Role: "bad", Document: "652.904.150-84",
	})
	assert.Error(t, err)
}

func TestDelete_NotFound(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo, &mockAuthClient{}, &mockHasher{})
	id := uuid.New()
	repo.On("FindByID", mock.Anything, id).Return(nil, domain.ErrUserNotFound)
	_, err := svc.Delete(context.Background(), id)
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}
