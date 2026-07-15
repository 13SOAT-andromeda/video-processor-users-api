package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/config"
	httpAdapter "github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/http/handlers"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/services"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// --- in-memory repository ---

type inMemRepo struct {
	mu    sync.Mutex
	users map[uuid.UUID]*domain.User
}

func newInMemRepo() *inMemRepo {
	return &inMemRepo{users: make(map[uuid.UUID]*domain.User)}
}

func (r *inMemRepo) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	for _, existing := range r.users {
		if existing.Email == u.Email {
			return domain.ErrEmailAlreadyExists
		}
	}
	cp := *u
	r.users[u.ID] = &cp
	return nil
}

func (r *inMemRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

func (r *inMemRepo) Update(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *u
	r.users[u.ID] = &cp
	return nil
}

func (r *inMemRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.users, id)
	return nil
}

func (r *inMemRepo) List(_ context.Context, limit, offset int) ([]*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := make([]*domain.User, 0, len(r.users))
	for _, u := range r.users {
		cp := *u
		all = append(all, &cp)
	}
	if offset >= len(all) {
		return []*domain.User{}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

// --- no-op auth client ---

type noopAuth struct{}

func (n *noopAuth) CreateCredential(_ context.Context, _, _ string, _ uuid.UUID, _ domain.Role) error {
	return nil
}

// --- plain hasher for tests ---

type plainHasher struct{}

func (p *plainHasher) Hash(plain string) (string, error) { return "h:" + plain, nil }
func (p *plainHasher) Compare(hash, plain string) bool   { return hash == "h:"+plain }

// --- router factory ---

func setupRouter(t *testing.T) (*httptest.Server, *inMemRepo) {
	t.Helper()

	repo := newInMemRepo()
	svc := services.NewUserService(repo, &noopAuth{}, &plainHasher{})
	handler := handlers.NewUserHandler(svc)

	cfg := config.Config{
		Env:  "test",
		Http: &config.HttpConfig{AllowedOrigins: []string{"*"}, Port: "8080"},
		JWT:  &config.JWTConfig{Secret: testJWTSecret},
	}

	router := httpAdapter.NewRouter(cfg, zap.NewNop(), *handler)
	srv := httptest.NewServer(router.Engine)
	t.Cleanup(srv.Close)
	return srv, repo
}

// --- helpers ---

func adminBearer(t *testing.T) string {
	t.Helper()
	tok, err := generateJWT(testJWTSecret, "administrator")
	assert.NoError(t, err)
	return "Bearer " + tok
}

func userBearer(t *testing.T) string {
	t.Helper()
	tok, err := generateJWT(testJWTSecret, "user")
	assert.NoError(t, err)
	return "Bearer " + tok
}

func doJSON(method, url, bearer string, body interface{}) (*http.Response, error) {
	var b []byte
	var err error
	if body != nil {
		b, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	return http.DefaultClient.Do(req)
}

// --- tests ---

func TestHealth(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := http.Get(srv.URL + "/api/health")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListUsers_Unauthorized(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := http.Get(srv.URL + "/api/users")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestListUsers_ForbiddenForNonAdmin(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users", userBearer(t), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCreateUser_Success(t *testing.T) {
	srv, _ := setupRouter(t)

	resp, err := doJSON(http.MethodPost, srv.URL+"/api/users", adminBearer(t), map[string]string{
		"name": "John Doe", "email": "john@example.com",
		"password": "Admin@123456", "role": "user", "document": "652.904.150-84",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "success", result["status"])
	assert.NotEmpty(t, result["id"])
}

func TestCreateUser_InvalidPassword(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodPost, srv.URL+"/api/users", adminBearer(t), map[string]string{
		"name": "Bad", "email": "bad@example.com",
		"password": "weak", "role": "user", "document": "652.904.150-84",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	srv, _ := setupRouter(t)

	payload := map[string]string{
		"name": "User", "email": "dup@example.com",
		"password": "Admin@123456", "role": "user", "document": "652.904.150-84",
	}
	r1, err := doJSON(http.MethodPost, srv.URL+"/api/users", adminBearer(t), payload)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, r1.StatusCode)

	r2, err := doJSON(http.MethodPost, srv.URL+"/api/users", adminBearer(t), payload)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, r2.StatusCode)

	var errBody map[string]interface{}
	assert.NoError(t, json.NewDecoder(r2.Body).Decode(&errBody))
	assert.Equal(t, "EMAIL_ALREADY_EXISTS", errBody["code"])
}

func TestGetByID_NotFound(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users/"+uuid.New().String(), adminBearer(t), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetByID_Success(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "Alice", Email: "alice@x.com", Role: domain.RoleUser, Document: "652.904.150-84"}

	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users/"+id.String(), adminBearer(t), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var user ports.UserResponse
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&user))
	assert.Equal(t, "Alice", user.Name)
}

func TestUpdateUser_Success(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "Old", Email: "old@x.com", Role: domain.RoleUser, Document: "652.904.150-84"}

	resp, err := doJSON(http.MethodPut, srv.URL+"/api/users/"+id.String(), adminBearer(t), map[string]string{"name": "New Name"})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteUser_Success(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "ToDelete"}

	resp, err := doJSON(http.MethodDelete, srv.URL+"/api/users/"+id.String(), adminBearer(t), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteUser_NotFound(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodDelete, srv.URL+"/api/users/"+uuid.New().String(), adminBearer(t), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestListUsers_Success(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users", adminBearer(t), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
