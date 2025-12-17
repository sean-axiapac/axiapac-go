package security

import (
	"encoding/base64"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AxiapacIdentity struct {
	Id       int
	UserName string
	Provider string
	Email    string
}

// IdentityClaims includes Identity and standard JWT claims

type Identity struct {
	ID         int    `json:"nameid"`
	UniqueName string `json:"unique_name"`
	Email      string `json:"email"`
	SID        string `json:"sid"`
	Provider   string `json:"provider"`
}
type IdentityClaims struct {
	Identity
	jwt.RegisteredClaims
}

func CreateIdentityToken(identity *AxiapacIdentity, base64Secret string, expiresInSeconds int64) (string, error) {
	secretBytes, err := base64.StdEncoding.DecodeString(base64Secret)
	if err != nil {
		return "", err
	}
	claims := IdentityClaims{
		Identity: Identity{
			ID:         identity.Id,
			UniqueName: identity.UserName,
			Email:      identity.Email,
			SID:        "axgo-deviceId",
			Provider:   identity.Provider,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "axiapac",
			Audience:  []string{"*.axiapac.net.au"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiresInSeconds) * time.Second)),
		},
	}

	// Use HS256 signing method (symmetric key)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secretBytes))
}
