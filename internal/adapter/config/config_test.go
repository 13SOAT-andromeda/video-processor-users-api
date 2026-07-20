package config_test

import (
	"os"
	"testing"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/config"
	"github.com/stretchr/testify/assert"
)

func TestInit_Defaults(t *testing.T) {
	// unset any existing vars that might interfere
	os.Unsetenv("DB_HOST")
	os.Unsetenv("JWT_SIGNING_KEY_SECRET_NAME")
	os.Unsetenv("DD_AGENT_HOST")

	cfg, err := config.Init()
	assert.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "", cfg.JWT.SigningKeySecretName)
	assert.True(t, cfg.DogStatsD.Disabled)
}

func TestInit_WithEnvVars(t *testing.T) {
	os.Setenv("DB_HOST", "myhost")
	os.Setenv("JWT_SIGNING_KEY_SECRET_NAME", "jwt-signing-key-prod")
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("AUTH_SERVICE_URL", "http://auth:8081")
	os.Setenv("DD_AGENT_HOST", "dd-agent")
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("JWT_SIGNING_KEY_SECRET_NAME")
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("AUTH_SERVICE_URL")
		os.Unsetenv("DD_AGENT_HOST")
	}()

	cfg, err := config.Init()
	assert.NoError(t, err)
	assert.Equal(t, "myhost", cfg.Database.Host)
	assert.Equal(t, "jwt-signing-key-prod", cfg.JWT.SigningKeySecretName)
	assert.Equal(t, "9090", cfg.Http.Port)
	assert.Equal(t, "http://auth:8081", cfg.Auth.ServiceURL)
	assert.False(t, cfg.DogStatsD.Disabled)
}
