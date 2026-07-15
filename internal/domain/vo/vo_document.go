package vo

import (
	"errors"
	"regexp"
	"strconv"
)

type Document string

var reDoc = regexp.MustCompile(`\D`)

func NewDocument(raw string) (Document, error) {
	digits := reDoc.ReplaceAllString(raw, "")
	if !validCPF(digits) {
		return "", errors.New("invalid document: CPF checksum failed")
	}
	return Document(raw), nil
}

func (d Document) String() string { return string(d) }

func validCPF(digits string) bool {
	if len(digits) != 11 {
		return false
	}
	// all same digit is invalid
	allSame := true
	for i := 1; i < 11; i++ {
		if digits[i] != digits[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return false
	}

	sum := 0
	for i := 0; i < 9; i++ {
		n, _ := strconv.Atoi(string(digits[i]))
		sum += n * (10 - i)
	}
	r := (sum * 10) % 11
	if r == 10 {
		r = 0
	}
	d1, _ := strconv.Atoi(string(digits[9]))
	if r != d1 {
		return false
	}

	sum = 0
	for i := 0; i < 10; i++ {
		n, _ := strconv.Atoi(string(digits[i]))
		sum += n * (11 - i)
	}
	r = (sum * 10) % 11
	if r == 10 {
		r = 0
	}
	d2, _ := strconv.Atoi(string(digits[10]))
	return r == d2
}
