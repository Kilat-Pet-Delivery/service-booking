package booking

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_GenerateQRToken_Is32Chars_URLSafe(t *testing.T) {
	token, err := GenerateQRToken()
	require.NoError(t, err)
	require.Len(t, token, 32)
	for _, ch := range token {
		require.True(t, ch == '-' || ch == '_' || ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z')
	}
}

func Test_ValidateQRToken_Rejects_Mismatch(t *testing.T) {
	shopID := uuid.New()
	bk := mustShopBooking(t, shopID)
	require.NoError(t, bk.AcceptByShop(shopID))
	require.NoError(t, bk.MarkReadyForPickup(shopID, "right-token"))

	ok, err := ValidateQRToken(bk, "wrong-token")
	require.NoError(t, err)
	require.False(t, ok)
}
