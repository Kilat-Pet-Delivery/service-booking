package booking

import (
	"testing"

	"github.com/Kilat-Pet-Delivery/lib-proto/dto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_AcceptByShop_RequiresRequestedState(t *testing.T) {
	shopID := uuid.New()
	bk := mustShopBooking(t, shopID)
	require.NoError(t, bk.AcceptByShop(shopID))

	err := bk.AcceptByShop(shopID)
	require.Error(t, err)
}

func Test_MarkReadyForPickup_AcceptedFromBoth_AcceptedByShop_And_Preparing(t *testing.T) {
	shopID := uuid.New()

	fromAccepted := mustShopBooking(t, shopID)
	require.NoError(t, fromAccepted.AcceptByShop(shopID))
	require.NoError(t, fromAccepted.MarkReadyForPickup(shopID, "token-a"))
	require.Equal(t, StatusReadyForPickup, fromAccepted.Status())

	fromPreparing := mustShopBooking(t, shopID)
	require.NoError(t, fromPreparing.AcceptByShop(shopID))
	require.NoError(t, fromPreparing.MarkPreparing(shopID))
	require.NoError(t, fromPreparing.MarkReadyForPickup(shopID, "token-b"))
	require.Equal(t, StatusReadyForPickup, fromPreparing.Status())
}

func Test_NoShopID_PreservesExisting4StateFlow(t *testing.T) {
	bk := mustRunnerBooking(t)
	err := bk.AcceptByShop(uuid.New())
	require.Error(t, err)

	runnerID := uuid.New()
	require.NoError(t, bk.Accept(runnerID))
	require.Equal(t, StatusAccepted, bk.Status())
}

func mustShopBooking(t *testing.T, shopID uuid.UUID) *Booking {
	t.Helper()
	bk, err := NewBooking(
		uuid.New(),
		PetSpecification{PetType: string(PetTypeCat), Name: "Milo", WeightKg: 4},
		DetermineCrateRequirement(PetSpecification{PetType: string(PetTypeCat), Name: "Milo", WeightKg: 4}),
		dto.AddressDTO{Line1: "A", Latitude: 3.1, Longitude: 101.6},
		dto.AddressDTO{Line1: "B", Latitude: 3.2, Longitude: 101.7},
		1000,
		"MYR",
		nil,
		"",
		&shopID,
	)
	require.NoError(t, err)
	return bk
}

func mustRunnerBooking(t *testing.T) *Booking {
	t.Helper()
	bk, err := NewBooking(
		uuid.New(),
		PetSpecification{PetType: string(PetTypeCat), Name: "Milo", WeightKg: 4},
		DetermineCrateRequirement(PetSpecification{PetType: string(PetTypeCat), Name: "Milo", WeightKg: 4}),
		dto.AddressDTO{Line1: "A", Latitude: 3.1, Longitude: 101.6},
		dto.AddressDTO{Line1: "B", Latitude: 3.2, Longitude: 101.7},
		1000,
		"MYR",
		nil,
		"",
		nil,
	)
	require.NoError(t, err)
	return bk
}
