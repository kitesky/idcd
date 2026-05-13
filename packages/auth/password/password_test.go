package password

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		email       string
		wantErr     bool
		errMessage  string
	}{
		{
			name:     "valid password",
			password: "password123",
			email:    "user@example.com",
			wantErr:  false,
		},
		{
			name:       "too short",
			password:   "pass1",
			email:      "user@example.com",
			wantErr:    true,
			errMessage: "密码长度必须至少8字符",
		},
		{
			name:       "too long",
			password:   strings.Repeat("a", 73) + "1",
			email:      "user@example.com",
			wantErr:    true,
			errMessage: "密码长度不能超过72字符",
		},
		{
			name:       "no digits",
			password:   "onlyletters",
			email:      "user@example.com",
			wantErr:    true,
			errMessage: "密码必须包含字母和数字",
		},
		{
			name:       "no letters",
			password:   "12345678",
			email:      "user@example.com",
			wantErr:    true,
			errMessage: "密码必须包含字母和数字",
		},
		{
			name:       "same as email prefix",
			password:   "username123",
			email:      "username123@example.com",
			wantErr:    true,
			errMessage: "密码不能与邮箱前缀相同",
		},
		{
			name:       "case insensitive email prefix check",
			password:   "USERNAME123",
			email:      "username123@example.com",
			wantErr:    true,
			errMessage: "密码不能与邮箱前缀相同",
		},
		{
			name:     "empty email is ok",
			password: "password123",
			email:    "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password, tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMessage != "" {
				if !strings.Contains(err.Error(), tt.errMessage) {
					t.Errorf("ValidatePassword() error = %v, want error containing %v", err, tt.errMessage)
				}
			}
		})
	}
}

func TestHash(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "password123",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := Hash(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("Hash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Check hash format
				parts := strings.Split(hash, ":")
				if len(parts) != 2 {
					t.Errorf("Hash() returned invalid format, expected 'salt:key', got %v", hash)
				}
				// Each part should be valid base64
				if len(parts[0]) == 0 || len(parts[1]) == 0 {
					t.Errorf("Hash() returned empty salt or key")
				}
			}
		})
	}
}

func TestVerify(t *testing.T) {
	password := "test123password"
	hash, err := Hash(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{
			name:     "correct password",
			password: password,
			hash:     hash,
			want:     true,
		},
		{
			name:     "incorrect password",
			password: "wrongpassword",
			hash:     hash,
			want:     false,
		},
		{
			name:     "empty password",
			password: "",
			hash:     hash,
			want:     false,
		},
		{
			name:     "empty hash",
			password: password,
			hash:     "",
			want:     false,
		},
		{
			name:     "malformed hash",
			password: password,
			hash:     "invalid:format:with:colons",
			want:     false,
		},
		{
			name:     "invalid base64 in hash",
			password: password,
			hash:     "invalid_base64:another_invalid",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Verify(tt.password, tt.hash)
			if got != tt.want {
				t.Errorf("Verify() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashAndVerifyRoundTrip(t *testing.T) {
	passwords := []string{
		"simplepass123",
		"ComplexP@ssw0rd!",
		"中文密码123",
		"émojì🔑pass123",
		strings.Repeat("a", 50) + "123", // long password
	}

	for _, password := range passwords {
		t.Run("password_"+password[:min(10, len(password))], func(t *testing.T) {
			hash, err := Hash(password)
			if err != nil {
				t.Errorf("Hash() failed: %v", err)
				return
			}

			if !Verify(password, hash) {
				t.Error("Verify() failed for correct password")
			}

			// Test wrong password
			if Verify(password+"wrong", hash) {
				t.Error("Verify() succeeded for incorrect password")
			}
		})
	}
}

func TestHashUniqueness(t *testing.T) {
	password := "samepassword123"
	hash1, err1 := Hash(password)
	hash2, err2 := Hash(password)

	if err1 != nil || err2 != nil {
		t.Fatalf("Hash() failed: err1=%v, err2=%v", err1, err2)
	}

	if hash1 == hash2 {
		t.Error("Hash() returned identical hashes for same password (salt should make them different)")
	}

	// But both should verify correctly
	if !Verify(password, hash1) || !Verify(password, hash2) {
		t.Error("Verify() failed for hashed passwords")
	}
}

// Helper function for older Go versions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}