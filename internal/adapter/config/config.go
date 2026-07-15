package config

import (
	"net"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Database    *DataBaseConfig
	Http        *HttpConfig
	Auth        *AuthConfig
	AdminUser   *AdminUserConfig
	JWT         *JWTConfig
	DogStatsD   *DogStatsDConfig
	Env         string
	Version     string
	Service     string
}

type DataBaseConfig struct {
	Host     string
	User     string
	Password string
	DBName   string
	Port     string
	SSLMode  string
	TimeZone string
}

type HttpConfig struct {
	AllowedOrigins []string
	Port           string
}

type AuthConfig struct {
	ServiceURL string
}

type AdminUserConfig struct {
	Email    string
	Password string
	Document string
}

type JWTConfig struct {
	Secret string
}

type DogStatsDConfig struct {
	Addr     string
	Disabled bool
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func Init() (*Config, error) {
	_ = godotenv.Load()

	database := &DataBaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "password"),
		DBName:   getEnv("DB_NAME", "users_db"),
		Port:     getEnv("DB_PORT", "5432"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
		TimeZone: getEnv("DB_TIMEZONE", "UTC"),
	}

	allowedOrigins := getEnv("HTTP_ALLOWED_ORIGINS", "*")
	var originList []string
	if allowedOrigins == "*" {
		originList = []string{"*"}
	} else {
		originList = strings.Split(allowedOrigins, ",")
	}

	http := &HttpConfig{
		AllowedOrigins: originList,
		Port:           getEnv("HTTP_PORT", "8080"),
	}

	auth := &AuthConfig{
		ServiceURL: getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
	}

	adminUser := &AdminUserConfig{
		Email:    getEnv("ADMIN_EMAIL", ""),
		Password: getEnv("ADMIN_PASSWORD", ""),
		Document: getEnv("ADMIN_DOCUMENT", ""),
	}

	jwt := &JWTConfig{
		Secret: getEnv("JWT_SECRET", ""),
	}

	agentHost := getEnv("DD_AGENT_HOST", "")
	dogstatsdAddr := ""
	dogstatsdDisabled := true
	if agentHost != "" {
		dogstatsdAddr = net.JoinHostPort(agentHost, "8125")
		dogstatsdDisabled = false
	}

	return &Config{
		Database:  database,
		Http:      http,
		Auth:      auth,
		AdminUser: adminUser,
		JWT:       jwt,
		DogStatsD: &DogStatsDConfig{Addr: dogstatsdAddr, Disabled: dogstatsdDisabled},
		Env:       getEnv("ENV", "development"),
		Version:   getEnv("API_VERSION", "1.0.0"),
		Service:   getEnv("DD_SERVICE", "video-processor-users-api"),
	}, nil
}
