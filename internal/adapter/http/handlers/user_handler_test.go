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
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/middleware"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockService struct{ mock.Mock }

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
	r.GET("/users", h.List)
	r.GET("/users/:id", h.GetByID)
	r.PUT("/users/:id", h.Update)
	r.DELETE("/users/:id", h.Delete)
	return r
}

// injectClaims populates middleware context keys directly (simulates AuthRequired middleware)
func injectClaims(userID, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.ContextUserID, userID)
		c.Set(middleware.ContextUserRole, role)
		c.Next()
	}
}

func setupHandlerTestWithClaims(svc ports.UserService, userID, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handlers.NewUserHandler(svc)
	r.Use(injectClaims(userID, role))
	r.GET("/users", h.List)
	r.GET("/users/:id", h.GetByID)
	r.PUT("/users/:id", h.Update)
	r.DELETE("/users/:id", h.Delete)
	return r
}

func TestGetByID_Handler_Admin_CanReadAny(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("GetByID", mock.Anything, id).Return(&ports.UserResponse{
		ID: id, Name: "X", Email: "x@x.com", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil)

	r := setupHandlerTestWithClaims(svc, uuid.New().String(), string(domain.RoleAdministrator))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetByID_Handler_Owner_CanReadOwn(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("GetByID", mock.Anything, id).Return(&ports.UserResponse{
		ID: id, Name: "X", Email: "x@x.com", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil)

	r := setupHandlerTestWithClaims(svc, id.String(), string(domain.RoleUser))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetByID_Handler_NonOwnerNonAdmin_Forbidden(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()

	r := setupHandlerTestWithClaims(svc, uuid.New().String(), string(domain.RoleUser))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetByID_Handler_NotFound(t *testing.T) {
	svc := &mockService{}
	id := uuid.New()
	svc.On("GetByID", mock.Anything, id).Return(nil, domain.ErrUserNotFound)

	r := setupHandlerTestWithClaims(svc, id.String(), string(domain.RoleUser))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/"+id.String(), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetByID_Handler_InvalidUUID(t *testing.T) {
	svc := &mockService{}
	r := setupHandlerTestWithClaims(svc, uuid.New().String(), string(domain.RoleAdministrator))
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
