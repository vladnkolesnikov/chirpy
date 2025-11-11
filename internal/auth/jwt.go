package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := time.Now().UTC()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
		IssuedAt:  jwt.NewNumericDate(now),
	})

	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})

	tempUUID := uuid.UUID{}

	if err != nil {
		fmt.Printf("Error parsing token: %s\n", err)
		return tempUUID, err
	}

	subject, err := token.Claims.GetSubject()
	if err != nil {
		fmt.Printf("Error getting token's subject: %s\n", err)
		return tempUUID, err
	}

	if err := uuid.Validate(subject); err != nil {
		fmt.Printf("Error validation token from jwt subject: %s\n", err)
		return tempUUID, err
	}

	return uuid.MustParse(subject), nil
}
