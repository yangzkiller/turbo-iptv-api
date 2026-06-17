package util

import (
	"crypto/sha256"
	"encoding/hex"
)

func ContentKey(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:])
}
