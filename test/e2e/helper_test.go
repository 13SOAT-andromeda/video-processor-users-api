package e2e_test

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testJWTSecret = "test-secret"

func generateJWT(secret, role, sub string) (string, error) {
	claims := jwt.MapClaims{
		"email": "test@example.com",
		"role":  role,
		"sub":   sub,
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
