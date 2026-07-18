package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdministrator Role = "administrator"
	RoleUser          Role = "user"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailAlreadyExists = errors.New("email already registered")

type User struct {
	ID        uuid.UUID
	Name      string
	Email     string
	Document  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
