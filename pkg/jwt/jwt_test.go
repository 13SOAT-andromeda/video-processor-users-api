package jwt_test

import (
	"testing"
	"time"

	pkgjwt "github.com/13SOAT-andromeda/video-processor-users-api/pkg/jwt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func makeToken(secret, email, role string, exp time.Time) string {
	claims := jwt.MapClaims{
		"email": email,
		"role":  role,
		"sub":   "uid",
		"exp":   exp.Unix(),
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return tok
}

func TestParseToken_Valid(t *testing.T) {
	tok := makeToken("secret", "x@x.com", "administrator", time.Now().Add(time.Hour))
	claims, err := pkgjwt.ParseToken(tok, "secret")
	assert.NoError(t, err)
	assert.Equal(t, "administrator", claims.Role)
	assert.Equal(t, "x@x.com", claims.Email)
}

func TestParseToken_WrongSecret(t *testing.T) {
	tok := makeToken("secret", "x@x.com", "administrator", time.Now().Add(time.Hour))
	_, err := pkgjwt.ParseToken(tok, "wrong")
	assert.Error(t, err)
}

func TestParseToken_Expired(t *testing.T) {
	tok := makeToken("secret", "x@x.com", "administrator", time.Now().Add(-time.Hour))
	_, err := pkgjwt.ParseToken(tok, "secret")
	assert.Error(t, err)
}

func TestParseToken_Invalid(t *testing.T) {
	_, err := pkgjwt.ParseToken("not-a-jwt", "secret")
	assert.Error(t, err)
}
