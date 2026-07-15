package vo

import (
	"errors"
	"unicode"
)

type Password string

func NewPassword(raw string) (Password, error) {
	if len(raw) < 8 {
		return "", errors.New("password: minimum 8 characters required")
	}
	var hasUpper, hasNumber, hasSymbol bool
	for _, c := range raw {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsDigit(c):
			hasNumber = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSymbol = true
		}
	}
	if !hasUpper {
		return "", errors.New("password: must contain at least one uppercase letter")
	}
	if !hasNumber {
		return "", errors.New("password: must contain at least one number")
	}
	if !hasSymbol {
		return "", errors.New("password: must contain at least one symbol")
	}
	return Password(raw), nil
}

func (p Password) String() string { return string(p) }
