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

func (r *inMemRepo) Upsert(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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

// --- router factory ---

func setupRouter(t *testing.T) (*httptest.Server, *inMemRepo) {
	t.Helper()

	repo := newInMemRepo()
	svc := services.NewUserService(repo)
	handler := handlers.NewUserHandler(svc)

	cfg := config.Config{
		Env:  "test",
		Http: &config.HttpConfig{AllowedOrigins: []string{"*"}, Port: "8080"},
		JWT:  &config.JWTConfig{},
	}

	router := httpAdapter.NewRouter(cfg, zap.NewNop(), *handler, testJWTSecret)
	srv := httptest.NewServer(router.Engine)
	t.Cleanup(srv.Close)
	return srv, repo
}

// --- bearer helpers ---

func adminBearer(t *testing.T, sub string) string {
	t.Helper()
	tok, err := generateJWT(testJWTSecret, "administrator", sub)
	assert.NoError(t, err)
	return "Bearer " + tok
}

func userBearer(t *testing.T, sub string) string {
	t.Helper()
	tok, err := generateJWT(testJWTSecret, "user", sub)
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

// GET /users — listagem

func TestListUsers_Unauthorized(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := http.Get(srv.URL + "/api/users")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestListUsers_ForbiddenForNonAdmin(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users", userBearer(t, uuid.New().String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestListUsers_Success(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users", adminBearer(t, uuid.New().String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// GET /users/:id — leitura por posse ou admin

func TestGetByID_Unauthorized(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := http.Get(srv.URL + "/api/users/" + uuid.New().String())
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestGetByID_Admin_CanReadAnyProfile(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "Alice", Email: "alice@x.com", Document: "652.904.150-84"}

	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users/"+id.String(), adminBearer(t, uuid.New().String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "Alice", body["name"])
	assert.Equal(t, "652.904.150-84", body["document"])
	// regressão: role não deve aparecer na resposta (ADR-012)
	_, hasRole := body["role"]
	assert.False(t, hasRole, "role must not be present in the response")
}

func TestGetByID_Owner_CanReadOwnProfile(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "Bob", Email: "bob@x.com", Document: "652.904.150-84"}

	// sub do JWT == id do perfil
	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users/"+id.String(), userBearer(t, id.String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var user ports.UserResponse
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&user))
	assert.Equal(t, "Bob", user.Name)
}

func TestGetByID_NonOwnerNonAdmin_Forbidden(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "Carol", Email: "carol@x.com"}

	// sub do JWT é diferente do id do perfil e role não é admin
	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users/"+id.String(), userBearer(t, uuid.New().String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestGetByID_NotFound(t *testing.T) {
	srv, _ := setupRouter(t)
	id := uuid.New()
	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users/"+id.String(), adminBearer(t, uuid.New().String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetByID_InvalidUUID(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodGet, srv.URL+"/api/users/not-a-uuid", adminBearer(t, uuid.New().String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// PUT /users/:id — atualização administrativa

func TestUpdateUser_Success(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "Old", Email: "old@x.com", Document: "652.904.150-84"}

	resp, err := doJSON(http.MethodPut, srv.URL+"/api/users/"+id.String(), adminBearer(t, uuid.New().String()), map[string]string{
		"name":     "New Name",
		"document": "652.904.150-84",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	updated, _ := repo.FindByID(context.Background(), id)
	assert.Equal(t, "New Name", updated.Name)
}

func TestUpdateUser_EmailIsIgnored(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "Dave", Email: "dave@x.com", Document: "652.904.150-84"}

	// regressão: PUT não deve aceitar alteração de email
	resp, err := doJSON(http.MethodPut, srv.URL+"/api/users/"+id.String(), adminBearer(t, uuid.New().String()), map[string]string{
		"email": "hacker@x.com",
		"name":  "Dave",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	unchanged, _ := repo.FindByID(context.Background(), id)
	assert.Equal(t, "dave@x.com", unchanged.Email)
}

func TestUpdateUser_ForbiddenForNonAdmin(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "Eve", Email: "eve@x.com"}

	resp, err := doJSON(http.MethodPut, srv.URL+"/api/users/"+id.String(), userBearer(t, id.String()), map[string]string{"name": "X"})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpdateUser_NotFound(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodPut, srv.URL+"/api/users/"+uuid.New().String(), adminBearer(t, uuid.New().String()), map[string]string{"name": "X"})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// DELETE /users/:id — remoção administrativa

func TestDeleteUser_Success(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "ToDelete"}

	resp, err := doJSON(http.MethodDelete, srv.URL+"/api/users/"+id.String(), adminBearer(t, uuid.New().String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_, err = repo.FindByID(context.Background(), id)
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestDeleteUser_NotFound(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodDelete, srv.URL+"/api/users/"+uuid.New().String(), adminBearer(t, uuid.New().String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteUser_ForbiddenForNonAdmin(t *testing.T) {
	srv, repo := setupRouter(t)
	id := uuid.New()
	repo.users[id] = &domain.User{ID: id, Name: "Frank"}

	resp, err := doJSON(http.MethodDelete, srv.URL+"/api/users/"+id.String(), userBearer(t, id.String()), nil)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// POST /users — não existe mais

func TestCreateUser_RouteNotFound(t *testing.T) {
	srv, _ := setupRouter(t)
	resp, err := doJSON(http.MethodPost, srv.URL+"/api/users", adminBearer(t, uuid.New().String()), map[string]string{
		"name": "X", "email": "x@x.com", "document": "652.904.150-84",
	})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
