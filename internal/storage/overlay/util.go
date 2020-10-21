package overlay

import (
	"crypto/rand"
	"encoding/hex"
)

// generateID generates a random unique id.
func generateID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
