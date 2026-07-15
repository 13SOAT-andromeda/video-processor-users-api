package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/handlers"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockService struct{ mock.Mock }

func (m *mockService) Create(ctx context.Context, req ports.CreateUserRequest) (*ports.CreateUserResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.CreateUserResponse), args.Error(1)
}

func (m *mockService) GetByID(ctx context.Context, id uuid.UUID) (*ports.UserResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.UserResponse), args.Error(1)
}

func (m *mockService) Update(ctx context.Context, id uuid.UUID, req ports.UpdateUserRequest) (*ports.UpdateUserResponse, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.UpdateUserResponse), args.Error(1)
}

func (m *mockService) Delete(ctx context.Context, id uuid.UUID) (*ports.DeleteUserResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.DeleteUserResponse), args.Error(1)
}

func (m *mockService) List(ctx context.Context, limit, offset int) (*ports.UserListResponse, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.UserListResponse), args.Error(1)
}

func setupHandlerTest(svc ports.UserService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewUserHandler(svc)
	r.POST("/users", h.Create)
	r.GET("/users", h.List)
	r.GET("/users/:id", h.GetByID)
	r.PUT("/users/:id", h.Update)
	r.DELETE("/users/:id", h.Delete)
	return r
}

func TestCreate_Handler_Success(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("Create", mock.Anything, mock.Anything).Return(&ports.CreateUserResponse{
		Status: "success", Message: "created", ID: id,
	}, nil)

	r := setupHandlerTest(svc)
	body, _ := json.Marshal(map[string]string{
		"name": "A", "email": "a@b.com", "password": "p", "role": "user", "document": "d",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreate_Handler_InvalidPayload(t *testing.T) {
	svc := &mockService{}
	r := setupHandlerTest(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreate_Handler_EmailExists(t *testing.T) {
	svc := &mockService{}
	svc.On("Create", mock.Anything, mock.Anything).Return(nil, domain.ErrEmailAlreadyExists)

	r := setupHandlerTest(svc)
	body, _ := json.Marshal(map[string]string{
		"name": "A", "email": "a@b.com", "password": "p", "role": "user", "document": "d",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetByID_Handler_Success(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("GetByID", mock.Anything, id).Return(&ports.UserResponse{
		ID: id, Name: "X", Email: "x@x.com", Role: domain.RoleUser,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil)

	r := setupHandlerTest(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetByID_Handler_NotFound(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("GetByID", mock.Anything, id).Return(nil, domain.ErrUserNotFound)

	r := setupHandlerTest(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetByID_Handler_InvalidUUID(t *testing.T) {
	svc := &mockService{}
	r := setupHandlerTest(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/not-a-uuid", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdate_Handler_Success(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("Update", mock.Anything, id, mock.Anything).Return(&ports.UpdateUserResponse{
		Status: "success", Message: "updated", ID: id,
	}, nil)

	r := setupHandlerTest(svc)
	body, _ := json.Marshal(map[string]string{"name": "New"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/users/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdate_Handler_NotFound(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("Update", mock.Anything, id, mock.Anything).Return(nil, domain.ErrUserNotFound)

	r := setupHandlerTest(svc)
	body, _ := json.Marshal(map[string]string{"name": "X"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/users/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdate_Handler_EmailExists(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("Update", mock.Anything, id, mock.Anything).Return(nil, domain.ErrEmailAlreadyExists)

	r := setupHandlerTest(svc)
	body, _ := json.Marshal(map[string]string{"email": "taken@x.com"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/users/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDelete_Handler_Success(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("Delete", mock.Anything, id).Return(&ports.DeleteUserResponse{Status: "success"}, nil)

	r := setupHandlerTest(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDelete_Handler_NotFound(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("Delete", mock.Anything, id).Return(nil, domain.ErrUserNotFound)

	r := setupHandlerTest(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDelete_Handler_ServerError(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("Delete", mock.Anything, id).Return(nil, errors.New("db error"))

	r := setupHandlerTest(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestList_Handler_Success(t *testing.T) {
	svc := &mockService{}
	svc.On("List", mock.Anything, 20, 0).Return(&ports.UserListResponse{
		Users: []*ports.UserResponse{}, Limit: 20, Offset: 0,
	}, nil)

	r := setupHandlerTest(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestList_Handler_Error(t *testing.T) {
	svc := &mockService{}
	svc.On("List", mock.Anything, 20, 0).Return(nil, errors.New("db error"))

	r := setupHandlerTest(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
