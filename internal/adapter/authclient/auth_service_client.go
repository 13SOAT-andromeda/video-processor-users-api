package authclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
)

type client struct {
	baseURL    string
	httpClient *http.Client
}

func NewAuthServiceClient(baseURL string) ports.AuthServiceClient {
	return &client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type createCredentialRequest struct {
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
	UserID       string `json:"user_id"`
	Role         string `json:"role"`
}

func (c *client) CreateCredential(ctx context.Context, email, passwordHash string, userID uuid.UUID, role domain.Role) error {
	body := createCredentialRequest{
		Email:        email,
		PasswordHash: passwordHash,
		UserID:       userID.String(),
		Role:         string(role),
	}

	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("authclient: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/credentials", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("authclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authclient: execute request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 400 {
		return fmt.Errorf("authclient: authentication-api returned status %d", resp.StatusCode)
	}

	return nil
}
