package vo

import (
	"errors"
	"regexp"
)

type Email string

var reEmail = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func NewEmail(raw string) (Email, error) {
	if !reEmail.MatchString(raw) {
		return "", errors.New("email: invalid format")
	}
	return Email(raw), nil
}

func (e Email) String() string { return string(e) }
