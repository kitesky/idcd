package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func bodyHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// sign computes the HMAC-SHA256 signature for an API request.
// message = method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + bodyHashHex
//
// Field delimiters ("\n") are required to prevent hash-ambiguity attacks where
// two different (method, path, ...) tuples produce identical concatenated strings.
func sign(method, path, timestamp, nonce string, body []byte, secret []byte) string {
	bh := bodyHash(body)
	sep := []byte("\n")
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(method))
	mac.Write(sep)
	mac.Write([]byte(path))
	mac.Write(sep)
	mac.Write([]byte(timestamp))
	mac.Write(sep)
	mac.Write([]byte(nonce))
	mac.Write(sep)
	mac.Write([]byte(bh))
	return hex.EncodeToString(mac.Sum(nil))
}
