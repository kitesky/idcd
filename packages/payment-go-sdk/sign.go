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
// message = method + path + timestamp + nonce + bodyHashHex (no separators)
func sign(method, path, timestamp, nonce string, body []byte, secret []byte) string {
	bh := bodyHash(body)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(method))
	mac.Write([]byte(path))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(nonce))
	mac.Write([]byte(bh))
	return hex.EncodeToString(mac.Sum(nil))
}
