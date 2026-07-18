package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/services"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetByID_NotFound(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo)

	id := uuid.New()
	repo.On("FindByID", mock.Anything, id).Return(nil, domain.ErrUserNotFound)

	_, err := svc.GetByID(context.Background(), id)
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestGetByID_Success(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo)

	id := uuid.New()
	u := &domain.User{ID: id, Name: "Alice", Email: "alice@example.com", Document: "652.904.150-84", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)

	resp, err := svc.GetByID(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, id, resp.ID)
	assert.Equal(t, "Alice", resp.Name)
}

func TestDelete_Success(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo)

	id := uuid.New()
	u := &domain.User{ID: id}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)
	repo.On("Delete", mock.Anything, id).Return(nil)

	resp, err := svc.Delete(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestDelete_NotFound(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo)

	id := uuid.New()
	repo.On("FindByID", mock.Anything, id).Return(nil, domain.ErrUserNotFound)

	_, err := svc.Delete(context.Background(), id)
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestList_Success(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo)

	users := []*domain.User{
		{ID: uuid.New(), Name: "User1", Email: "u1@x.com", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	repo.On("List", mock.Anything, 10, 0).Return(users, nil)

	resp, err := svc.List(context.Background(), 10, 0)
	assert.NoError(t, err)
	assert.Len(t, resp.Users, 1)
}

func TestUpdate_Success(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo)

	id := uuid.New()
	u := &domain.User{ID: id, Name: "Old", Email: "old@x.com", Document: "652.904.150-84"}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	req := ports.UpdateUserRequest{Name: "New", Document: "652.904.150-84"}
	resp, err := svc.Update(context.Background(), id, req)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}

func TestUpdate_NotFound(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo)

	id := uuid.New()
	repo.On("FindByID", mock.Anything, id).Return(nil, domain.ErrUserNotFound)

	_, err := svc.Update(context.Background(), id, ports.UpdateUserRequest{Name: "X"})
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestUpdate_OnlyName(t *testing.T) {
	repo := &mockUserRepository{}
	svc := services.NewUserService(repo)

	id := uuid.New()
	u := &domain.User{ID: id, Name: "Old", Email: "old@x.com", Document: "652.904.150-84"}
	repo.On("FindByID", mock.Anything, id).Return(u, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(updated *domain.User) bool {
		return updated.Name == "New" && updated.Document == "652.904.150-84"
	})).Return(nil)

	resp, err := svc.Update(context.Background(), id, ports.UpdateUserRequest{Name: "New"})
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
}
