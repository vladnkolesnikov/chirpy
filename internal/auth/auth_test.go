package auth

import "testing"

func TestHashPassword(t *testing.T) {
	password1 := "correctPassword123!"
	password2 := "anotherPassword456!"
	hash1, _ := HashPassword(password1)
	hash2, _ := HashPassword(password2)

	tests := []struct {
		name          string
		password      string
		hash          string
		matchPassword bool
		wantErr       bool
	}{
		{
			name:          "Correct password",
			password:      password1,
			hash:          hash1,
			wantErr:       false,
			matchPassword: true,
		},
		{
			name:          "Incorrect password",
			password:      password2,
			hash:          hash1,
			wantErr:       false,
			matchPassword: false,
		},
		{
			name:          "Password doesn't match different hash",
			password:      password1,
			hash:          hash2,
			wantErr:       false,
			matchPassword: false,
		},
		{
			name:          "Empty password",
			password:      "",
			hash:          hash1,
			wantErr:       false,
			matchPassword: false,
		},
		{
			name:          "Invalid hash",
			password:      password1,
			hash:          "invalidhash",
			wantErr:       true,
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
