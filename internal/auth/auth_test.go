package auth

import "testing"

func TestHashPassword(t *testing.T) {
	password := "test_password"
	hash, _ := HashPassword(password)

	tests := []struct {
		name          string
		password      string
		hash          string
		matchPassword bool
		wantErr       bool
	}{
		{
			name:          "Correct password",
			password:      password,
			hash:          hash,
			wantErr:       false,
			matchPassword: true,
		},
		{
			name:          "Incorrect password",
			password:      "wrongPassword",
			hash:          hash,
			wantErr:       false,
			matchPassword: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			match, err := CheckPasswordHash(test.password, test.hash)

			if (err != nil) != test.wantErr {
				t.Errorf("CheckPasswordHash() error = %v, wantErr %v", err, test.wantErr)
			}

			if !test.wantErr && match != test.matchPassword {
				t.Errorf("CheckPasswordHash() expects %v, got %v", test.matchPassword, match)
			}
		})
	}
}
