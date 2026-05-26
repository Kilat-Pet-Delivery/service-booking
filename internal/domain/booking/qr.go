package booking

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/Kilat-Pet-Delivery/lib-common/domain"
)

// GenerateQRToken returns a 32-character URL-safe pickup nonce.
func GenerateQRToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ValidateQRToken compares the supplied pickup token with the booking's active token.
func ValidateQRToken(b *Booking, token string) (bool, error) {
	if b == nil {
		return false, domain.NewValidationError("booking is required")
	}
	if token == "" {
		return false, domain.NewValidationError("token is required")
	}
	if b.qrPickupToken == nil {
		return false, nil
	}
	return *b.qrPickupToken == token, nil
}
