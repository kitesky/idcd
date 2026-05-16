package password

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name       string
		password   string
		email      string
		wantErr    bool
		errMessage string
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
			name:       "too long (over 1024 bytes)",
			password:   strings.Repeat("a", 1024) + "1",
			email:      "user@example.com",
			wantErr:    true,
			errMessage: fmt.Sprintf("密码长度不能超过%d字符", maxLength),
		},
		{
			name:     "long but within limit (200 chars) is allowed (bcrypt 72 no longer applies)",
			password: strings.Repeat("a", 200) + "1",
			email:    "user@example.com",
			wantErr:  false,
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

func TestHash_PHCFormat(t *testing.T) {
	hash, err := Hash("password123")
	if err != nil {
		t.Fatalf("Hash() unexpected error: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Fatalf("Hash() returned non-PHC format: %q", hash)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("PHC hash should split into 6 segments, got %d: %q", len(parts), hash)
	}
	// Param section must contain m=, t=, p=.
	if !strings.Contains(parts[3], "m=") || !strings.Contains(parts[3], "t=") || !strings.Contains(parts[3], "p=") {
		t.Fatalf("PHC param section malformed: %q", parts[3])
	}
}

func TestHash_EmptyPasswordRejected(t *testing.T) {
	if _, err := Hash(""); err == nil {
		t.Fatalf("Hash(\"\") expected error, got nil")
	}
}

func TestVerify_NewFormatRoundTrip(t *testing.T) {
	password := "test123password"
	hash, err := Hash(password)
	if err != nil {
		t.Fatalf("Hash() failed: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
	}{
		{"correct password", password, hash, true},
		{"wrong password", "wrongpassword", hash, false},
		{"empty password", "", hash, false},
		{"empty hash", password, "", false},
		{"phc with bad version", password, "$argon2id$v=18$m=65536,t=3,p=4$YWJjZGVmZ2hpamtsbW5vcA$" + base64.RawStdEncoding.EncodeToString(make([]byte, 32)), false},
		{"phc with bad algo", password, "$argon2i$v=19$m=65536,t=3,p=4$YWJjZGVmZ2hpamtsbW5vcA$" + base64.RawStdEncoding.EncodeToString(make([]byte, 32)), false},
		{"phc missing params", password, "$argon2id$v=19$m=65536,t=3$YWJjZGVmZ2hpamtsbW5vcA$" + base64.RawStdEncoding.EncodeToString(make([]byte, 32)), false},
		{"phc bad base64 salt", password, "$argon2id$v=19$m=65536,t=3,p=4$!!!notbase64!!!$" + base64.RawStdEncoding.EncodeToString(make([]byte, 32)), false},
		{"phc bad base64 key", password, "$argon2id$v=19$m=65536,t=3,p=4$YWJjZGVmZ2hpamtsbW5vcA$!!!notbase64!!!", false},
		{"phc truncated", password, "$argon2id$v=19$m=65536,t=3,p=4", false},
		{"completely malformed", password, "not-a-hash-at-all", false},
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

func TestVerify_MalformedDoesNotPanic(t *testing.T) {
	// A variety of malformed inputs must not panic and must return false.
	bad := []string{
		"$argon2id$",
		"$argon2id$v=19",
		"$argon2id$v=abc$m=65536,t=3,p=4$YWJj$YWJj",
		"$argon2id$v=19$m=0,t=3,p=4$YWJj$YWJj",
		"$argon2id$v=19$m=65536,t=0,p=4$YWJj$YWJj",
		"$argon2id$v=19$m=65536,t=3,p=0$YWJj$YWJj",
		"$argon2id$v=19$x=65536,t=3,p=4$YWJj$YWJj",
		"$argon2id$v=19$m=foo,t=3,p=4$YWJj$YWJj",
		"$argon2id$v=19$m=65536,t=3,p=4$$YWJj",
		"$argon2id$v=19$m=65536,t=3,p=4$YWJj$",
	}
	for _, h := range bad {
		t.Run(h, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Verify(%q) panicked: %v", h, r)
				}
			}()
			if Verify("anyPassword123", h) {
				t.Fatalf("Verify(%q) returned true for malformed hash", h)
			}
		})
	}
}

// TestVerify_LegacyFormatCompat ensures pre-PHC stored hashes still verify.
// Legacy format is base64rawurl(salt) ":" base64rawurl(key) with Argon2id
// using DefaultParams().
func TestVerify_LegacyFormatCompat(t *testing.T) {
	password := "legacyPassword123"

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("rand: %v", err)
	}
	p := DefaultParams()
	key := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Parallelism, p.KeyLen)
	legacy := base64.RawURLEncoding.EncodeToString(salt) + ":" + base64.RawURLEncoding.EncodeToString(key)

	if !Verify(password, legacy) {
		t.Fatalf("Verify() failed for valid legacy hash")
	}
	if Verify("wrongpassword", legacy) {
		t.Fatalf("Verify() succeeded for wrong password on legacy hash")
	}
	if Verify(password, "garbage:also-garbage") {
		t.Fatalf("Verify() succeeded for malformed legacy hash")
	}
	if Verify(password, "only-one-segment") {
		t.Fatalf("Verify() succeeded for legacy hash without colon")
	}
}

// TestVerify_DifferentParams ensures verify uses the parameters embedded in
// the stored hash, not the package defaults. Tests permutations of m/t/p.
func TestVerify_DifferentParams(t *testing.T) {
	password := "matrixPassword123"
	paramSets := []Params{
		{Memory: 8 * 1024, Time: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32},
		{Memory: 32 * 1024, Time: 2, Parallelism: 2, SaltLen: 16, KeyLen: 32},
		{Memory: 64 * 1024, Time: 3, Parallelism: 4, SaltLen: 16, KeyLen: 32},
		{Memory: 16 * 1024, Time: 4, Parallelism: 1, SaltLen: 24, KeyLen: 48},
	}
	for _, p := range paramSets {
		t.Run(fmt.Sprintf("m=%d,t=%d,p=%d", p.Memory, p.Time, p.Parallelism), func(t *testing.T) {
			hash, err := HashWithParams(password, p)
			if err != nil {
				t.Fatalf("HashWithParams: %v", err)
			}
			if !strings.HasPrefix(hash, phcPrefix) {
				t.Fatalf("HashWithParams returned non-PHC: %s", hash)
			}
			if !Verify(password, hash) {
				t.Fatalf("Verify failed for params %+v", p)
			}
			if Verify("wrongPassword123", hash) {
				t.Fatalf("Verify succeeded with wrong password for params %+v", p)
			}
		})
	}
}

func TestHashUniqueness(t *testing.T) {
	password := "samepassword123"
	hash1, err1 := Hash(password)
	hash2, err2 := Hash(password)
	if err1 != nil || err2 != nil {
		t.Fatalf("Hash() failed: %v / %v", err1, err2)
	}
	if hash1 == hash2 {
		t.Errorf("Hash() returned identical hashes for same password (salt should randomize)")
	}
	if !Verify(password, hash1) || !Verify(password, hash2) {
		t.Errorf("Verify failed on a freshly produced hash")
	}
}

func TestHashAndVerifyRoundTrip(t *testing.T) {
	passwords := []string{
		"simplepass123",
		"ComplexP@ssw0rd!",
		"中文密码123",
		"émojì🔑pass123",
		strings.Repeat("a", 50) + "123",
		// >72 bytes — old bcrypt limit; must work now.
		strings.Repeat("x", 200) + "abc123",
	}

	for _, password := range passwords {
		label := password
		if len(label) > 10 {
			label = label[:10]
		}
		t.Run("password_"+label, func(t *testing.T) {
			hash, err := Hash(password)
			if err != nil {
				t.Fatalf("Hash() failed: %v", err)
			}
			if !Verify(password, hash) {
				t.Errorf("Verify() failed for correct password")
			}
			if Verify(password+"wrong", hash) {
				t.Errorf("Verify() succeeded for incorrect password")
			}
		})
	}
}

// TestHash_LongPasswordNotTruncated ensures the bcrypt 72-byte cap is gone.
// Two 1KB-passwords that agree only past byte 72 must produce different hashes
// and must each fail to verify the other.
func TestHash_LongPasswordNotTruncated(t *testing.T) {
	prefix := strings.Repeat("a", 72)
	longA := prefix + strings.Repeat("b", 1024-72) // 1024 bytes total, "b" tail
	longB := prefix + strings.Repeat("c", 1024-72) // 1024 bytes total, "c" tail

	hashA, err := Hash(longA)
	if err != nil {
		t.Fatalf("Hash(longA) failed: %v (maxLength may be too low)", err)
	}
	hashB, err := Hash(longB)
	if err != nil {
		t.Fatalf("Hash(longB) failed: %v", err)
	}

	if !Verify(longA, hashA) {
		t.Errorf("Verify(longA, hashA) = false; long passwords must round-trip")
	}
	if !Verify(longB, hashB) {
		t.Errorf("Verify(longB, hashB) = false; long passwords must round-trip")
	}

	// Cross-verify: if the implementation silently truncated to 72 bytes the
	// two passwords would collide. They must not.
	if Verify(longA, hashB) {
		t.Errorf("Verify(longA, hashB) = true; password appears to be truncated")
	}
	if Verify(longB, hashA) {
		t.Errorf("Verify(longB, hashA) = true; password appears to be truncated")
	}
}

func TestNeedsRehash(t *testing.T) {
	current := DefaultParams()

	freshHash, err := HashWithParams("password123", current)
	if err != nil {
		t.Fatalf("HashWithParams: %v", err)
	}

	weakerParams := Params{Memory: 8 * 1024, Time: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}
	weakerHash, err := HashWithParams("password123", weakerParams)
	if err != nil {
		t.Fatalf("HashWithParams (weaker): %v", err)
	}

	// Build a legacy hash for the test.
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("rand: %v", err)
	}
	key := argon2.IDKey([]byte("password123"), salt, current.Time, current.Memory, current.Parallelism, current.KeyLen)
	legacy := base64.RawURLEncoding.EncodeToString(salt) + ":" + base64.RawURLEncoding.EncodeToString(key)

	tests := []struct {
		name   string
		stored string
		want   bool
	}{
		{"fresh hash with current params", freshHash, false},
		{"hash with weaker params should be rehashed", weakerHash, true},
		{"legacy format always needs rehash", legacy, true},
		{"empty stored needs rehash", "", true},
		{"malformed PHC needs rehash", "$argon2id$bogus", true},
		{"wrong algo needs rehash", "$argon2i$v=19$m=65536,t=3,p=4$YWJj$YWJj", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsRehash(tt.stored, current); got != tt.want {
				t.Errorf("NeedsRehash(%q) = %v, want %v", tt.stored, got, tt.want)
			}
		})
	}

	// Different KeyLen also triggers rehash.
	t.Run("different key length needs rehash", func(t *testing.T) {
		p := current
		p.KeyLen = 48
		h, err := HashWithParams("password123", p)
		if err != nil {
			t.Fatalf("HashWithParams: %v", err)
		}
		if !NeedsRehash(h, current) {
			t.Errorf("NeedsRehash should be true when KeyLen differs")
		}
	})
}

// TestHashWithParams_RejectsZeroParams guards against silently producing weak
// hashes when callers pass an unconfigured Params{}.
func TestHashWithParams_RejectsZeroParams(t *testing.T) {
	cases := []Params{
		{},
		{Memory: 0, Time: 3, Parallelism: 4, SaltLen: 16, KeyLen: 32},
		{Memory: 64 * 1024, Time: 0, Parallelism: 4, SaltLen: 16, KeyLen: 32},
		{Memory: 64 * 1024, Time: 3, Parallelism: 0, SaltLen: 16, KeyLen: 32},
		{Memory: 64 * 1024, Time: 3, Parallelism: 4, SaltLen: 0, KeyLen: 32},
		{Memory: 64 * 1024, Time: 3, Parallelism: 4, SaltLen: 16, KeyLen: 0},
	}
	for i, p := range cases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			if _, err := HashWithParams("password123", p); err == nil {
				t.Fatalf("HashWithParams should reject zero param: %+v", p)
			}
		})
	}
}
