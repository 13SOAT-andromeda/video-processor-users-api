package authclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/authclient"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCreateCredential_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/credentials", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := authclient.NewAuthServiceClient(srv.URL)
	err := client.CreateCredential(context.Background(), "test@example.com", "hash", uuid.New(), domain.RoleUser)
	assert.NoError(t, err)
}

func TestCreateCredential_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := authclient.NewAuthServiceClient(srv.URL)
	err := client.CreateCredential(context.Background(), "test@example.com", "hash", uuid.New(), domain.RoleUser)
	assert.Error(t, err)
}

func TestCreateCredential_BadURL(t *testing.T) {
	client := authclient.NewAuthServiceClient("http://localhost:0")
	err := client.CreateCredential(context.Background(), "test@example.com", "hash", uuid.New(), domain.RoleUser)
	assert.Error(t, err)
}
