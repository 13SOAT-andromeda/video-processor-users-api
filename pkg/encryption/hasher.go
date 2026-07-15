package encryption

import "golang.org/x/crypto/bcrypt"

type Hasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) bool
}

type BcryptHasher struct{}

func NewBcryptHasher() Hasher {
	return &BcryptHasher{}
}

func (b *BcryptHasher) Hash(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (b *BcryptHasher) Compare(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
