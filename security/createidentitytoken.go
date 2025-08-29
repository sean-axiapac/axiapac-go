package security

import (
	"encoding/base64"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Identity struct {
	Id       int
	UserName string
	Provider string
	Email    string
}

// IdentityClaims includes Identity and standard JWT claims
type IdentityClaims struct {
	Identity
	jwt.RegisteredClaims
}

func CreateIdentityToken(identity *Identity, base64Secret string, expiresInSeconds int64) (string, error) {
	secretBytes, err := base64.StdEncoding.DecodeString(base64Secret)
	if err != nil {
		return "", err
	}
	claims := IdentityClaims{
		Identity: *identity,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiresInSeconds) * time.Second)),
		},
	}

	// Use HS256 signing method (symmetric key)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secretBytes))
}
