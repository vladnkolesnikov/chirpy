package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateJWT(t *testing.T) {
	const secret = "secret"
	const secret2 = "secret2"

	userID, _ := uuid.NewUUID()

	tests := []struct {
		name           string
		wantErr        bool
		secret         string
		validateSecret string
		duration       time.Duration
	}{
		{
			name:           "Correct token",
			wantErr:        false,
			secret:         secret,
			validateSecret: secret,
			duration:       time.Duration(2) * time.Hour,
		},
		{
			name:           "Incorrect secret",
			wantErr:        true,
			secret:         secret,
			validateSecret: secret2,
			duration:       time.Duration(2) * time.Hour,
		},
		{
			name:           "Expired token",
			wantErr:        true,
			secret:         secret,
			validateSecret: secret,
			duration:       time.Duration(-1) * time.Hour,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token, _ := MakeJWT(userID, test.secret, test.duration)
			tokenUUID, err := ValidateJWT(token, test.validateSecret)

			if (err != nil) != test.wantErr {
				t.Errorf("ValidateJWT() error = %s, wantErr %t", err, test.wantErr)
			}

			if !test.wantErr && tokenUUID != userID {
				t.Errorf("ValidateJWT() expects %s, got %s", userID, tokenUUID)
			}
		})
	}
}
