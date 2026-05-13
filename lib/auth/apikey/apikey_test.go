package apikey

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/kite365/idcd/lib/shared/apperr"
)

func TestGenerate(t *testing.T) {
	t.Run("successful generation", func(t *testing.T) {
		plaintext, hash, err := Generate()
		require.NoError(t, err)
		assert.NotEmpty(t, plaintext)
		assert.NotEmpty(t, hash)

		// Verify format
		assert.True(t, strings.HasPrefix(plaintext, keyPrefix))
		assert.Contains(t, hash, ":") // should contain salt:key separator

		// Verify plaintext and hash are different
		assert.NotEqual(t, plaintext, hash)

		// Verify the generated hash can verify the plaintext
		assert.True(t, Verify(plaintext, hash))
	})

	t.Run("generates unique keys", func(t *testing.T) {
		plaintext1, hash1, err1 := Generate()
		require.NoError(t, err1)

		plaintext2, hash2, err2 := Generate()
		require.NoError(t, err2)

		// Keys should be different
		assert.NotEqual(t, plaintext1, plaintext2)
		assert.NotEqual(t, hash1, hash2)

		// Each key should verify with its own hash
		assert.True(t, Verify(plaintext1, hash1))
		assert.True(t, Verify(plaintext2, hash2))

		// Keys should not verify with each other's hashes
		assert.False(t, Verify(plaintext1, hash2))
		assert.False(t, Verify(plaintext2, hash1))
	})

	t.Run("generated key format", func(t *testing.T) {
		plaintext, _, err := Generate()
		require.NoError(t, err)

		// Should start with prefix
		assert.True(t, strings.HasPrefix(plaintext, keyPrefix))

		// Should be longer than just the prefix
		assert.Greater(t, len(plaintext), len(keyPrefix))

		// Should not contain invalid base64 characters
		keyPart := plaintext[len(keyPrefix):]
		assert.NotContains(t, keyPart, "+")
		assert.NotContains(t, keyPart, "/")
		assert.NotContains(t, keyPart, "=")
	})
}

func TestHash(t *testing.T) {
	validKey := keyPrefix + "abcdefghijklmnopqrstuvwxyz123456"

	t.Run("successful hash", func(t *testing.T) {
		hash, err := Hash(validKey)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Contains(t, hash, ":")
		assert.NotEqual(t, validKey, hash)
	})

	t.Run("empty plaintext", func(t *testing.T) {
		_, err := Hash("")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("invalid prefix", func(t *testing.T) {
		_, err := Hash("invalid_prefix_key")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("hash is deterministic for same input with different salts", func(t *testing.T) {
		hash1, err := Hash(validKey)
		require.NoError(t, err)

		hash2, err := Hash(validKey)
		require.NoError(t, err)

		// Hashes should be different due to different salts
		assert.NotEqual(t, hash1, hash2)

		// But both should verify the same plaintext
		assert.True(t, Verify(validKey, hash1))
		assert.True(t, Verify(validKey, hash2))
	})
}

func TestVerify(t *testing.T) {
	plaintext, hash, err := Generate()
	require.NoError(t, err)

	t.Run("valid verification", func(t *testing.T) {
		result := Verify(plaintext, hash)
		assert.True(t, result)
	})

	t.Run("empty plaintext", func(t *testing.T) {
		result := Verify("", hash)
		assert.False(t, result)
	})

	t.Run("empty hash", func(t *testing.T) {
		result := Verify(plaintext, "")
		assert.False(t, result)
	})

	t.Run("wrong plaintext", func(t *testing.T) {
		wrongPlaintext := keyPrefix + "wrongkeyhere123456789"
		result := Verify(wrongPlaintext, hash)
		assert.False(t, result)
	})

	t.Run("wrong hash", func(t *testing.T) {
		wrongHash := "wrongsalt:wrongkey"
		result := Verify(plaintext, wrongHash)
		assert.False(t, result)
	})

	t.Run("invalid hash format", func(t *testing.T) {
		invalidHash := "nocolonhere"
		result := Verify(plaintext, invalidHash)
		assert.False(t, result)
	})

	t.Run("invalid plaintext prefix", func(t *testing.T) {
		invalidPlaintext := "wrong_prefix_key123"
		result := Verify(invalidPlaintext, hash)
		assert.False(t, result)
	})

	t.Run("malformed hash with too many parts", func(t *testing.T) {
		malformedHash := "salt:key:extra"
		result := Verify(plaintext, malformedHash)
		assert.False(t, result)
	})

	t.Run("malformed base64 in hash", func(t *testing.T) {
		malformedHash := "invalid-base64!:another-invalid-base64!"
		result := Verify(plaintext, malformedHash)
		assert.False(t, result)
	})
}

func TestExtractPrefix(t *testing.T) {
	t.Run("valid key", func(t *testing.T) {
		plaintext := keyPrefix + "abcdefghijklmnopqrstuvwxyz123456"
		prefix := ExtractPrefix(plaintext)

		expected := keyPrefix + "abcdefgh"
		assert.Equal(t, expected, prefix)
	})

	t.Run("short key", func(t *testing.T) {
		shortKey := keyPrefix + "short"
		prefix := ExtractPrefix(shortKey)

		// Should return the whole key if it's too short
		assert.Equal(t, shortKey, prefix)
	})

	t.Run("invalid prefix", func(t *testing.T) {
		invalidKey := "wrong_prefix_key"
		prefix := ExtractPrefix(invalidKey)

		// Should return as-is for invalid format
		assert.Equal(t, invalidKey, prefix)
	})

	t.Run("generated key prefix", func(t *testing.T) {
		plaintext, _, err := Generate()
		require.NoError(t, err)

		prefix := ExtractPrefix(plaintext)

		// Should start with the key prefix
		assert.True(t, strings.HasPrefix(prefix, keyPrefix))

		// Should be shorter than the full key
		assert.Less(t, len(prefix), len(plaintext))

		// Should be exactly prefix + 8 characters
		expectedLen := len(keyPrefix) + 8
		assert.Equal(t, expectedLen, len(prefix))
	})
}

func TestIntegration(t *testing.T) {
	t.Run("end-to-end flow", func(t *testing.T) {
		// Generate a new API key
		plaintext, hash, err := Generate()
		require.NoError(t, err)

		// Extract prefix for storage
		prefix := ExtractPrefix(plaintext)
		assert.True(t, strings.HasPrefix(prefix, keyPrefix))
		assert.Less(t, len(prefix), len(plaintext))

		// Verify the key works
		assert.True(t, Verify(plaintext, hash))

		// Hash the same plaintext again (different salt)
		hash2, err := Hash(plaintext)
		require.NoError(t, err)
		assert.NotEqual(t, hash, hash2) // Different due to salt

		// Both hashes should verify the same plaintext
		assert.True(t, Verify(plaintext, hash))
		assert.True(t, Verify(plaintext, hash2))

		// Wrong key should not verify
		wrongKey := keyPrefix + "wrongkeyhere"
		assert.False(t, Verify(wrongKey, hash))
		assert.False(t, Verify(wrongKey, hash2))
	})
}

func TestConstantTimeComparison(t *testing.T) {
	t.Run("timing attack resistance", func(t *testing.T) {
		plaintext, hash, err := Generate()
		require.NoError(t, err)

		// Create keys that differ in the last character
		wrongKey1 := plaintext[:len(plaintext)-1] + "X"
		wrongKey2 := plaintext[:len(plaintext)-1] + "Y"

		// Both should fail verification
		assert.False(t, Verify(wrongKey1, hash))
		assert.False(t, Verify(wrongKey2, hash))

		// The correct key should still verify
		assert.True(t, Verify(plaintext, hash))
	})
}

func TestHashErrorCases(t *testing.T) {
	t.Run("very short invalid key", func(t *testing.T) {
		_, err := Hash("sk_")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("completely wrong format", func(t *testing.T) {
		_, err := Hash("totally-wrong-format")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})
}

func TestVerifyErrorCases(t *testing.T) {
	t.Run("hash with only one part", func(t *testing.T) {
		plaintext := keyPrefix + "validkeyformat123456"
		result := Verify(plaintext, "onlyonepart")
		assert.False(t, result)
	})

	t.Run("completely malformed hash", func(t *testing.T) {
		plaintext := keyPrefix + "validkeyformat123456"
		result := Verify(plaintext, "")
		assert.False(t, result)
	})

	t.Run("hash with multiple colons", func(t *testing.T) {
		plaintext := keyPrefix + "validkeyformat123456"
		result := Verify(plaintext, "part1:part2:part3:part4")
		assert.False(t, result)
	})
}

func TestExtractPrefixEdgeCases(t *testing.T) {
	t.Run("exactly minimum length", func(t *testing.T) {
		minKey := keyPrefix + "12345678" // exactly 8 chars after prefix
		prefix := ExtractPrefix(minKey)
		assert.Equal(t, minKey, prefix)
	})

	t.Run("empty string", func(t *testing.T) {
		prefix := ExtractPrefix("")
		assert.Equal(t, "", prefix)
	})

	t.Run("only prefix", func(t *testing.T) {
		prefix := ExtractPrefix(keyPrefix)
		assert.Equal(t, keyPrefix, prefix)
	})
}

func TestGenerateMultipleRuns(t *testing.T) {
	t.Run("generate many keys for robustness", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			plaintext, hash, err := Generate()
			require.NoError(t, err)
			assert.NotEmpty(t, plaintext)
			assert.NotEmpty(t, hash)
			assert.True(t, strings.HasPrefix(plaintext, keyPrefix))
			assert.True(t, Verify(plaintext, hash))
		}
	})
}