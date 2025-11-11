package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
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
		return tempUUID, fmt.Errorf("Error parsing token: %s\n", err)
	}

	userID, err := token.Claims.GetSubject()
	if err != nil {
		return tempUUID, fmt.Errorf("Error getting subject: %s\n", err)
	}

	if err = uuid.Validate(userID); err != nil {
		return tempUUID, fmt.Errorf("Error validating userID: %s\n", err)
	}

	return uuid.MustParse(userID), nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization token is missing")
	}

	return strings.Replace(authHeader, "Bearer ", "", 1), nil
}
