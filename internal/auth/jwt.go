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

const ISSUER = "chirpy"

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := time.Now().UTC()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    ISSUER,
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

	if err != nil {
		return uuid.Nil, fmt.Errorf("Error parsing token: %s\n", err)
	}

	userID, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.Nil, fmt.Errorf("Error getting subject: %s\n", err)
	}

	issuer, err := token.Claims.GetIssuer()
	if err != nil {
		return uuid.Nil, err
	}

	if issuer != ISSUER {
		return uuid.Nil, errors.New("invalid issuer")
	}

	id, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("Error validating userID: %s\n", err)
	}

	return id, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization token is missing")
	}

	return strings.Replace(authHeader, "Bearer ", "", 1), nil
}
