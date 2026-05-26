//go:build integration

package main_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-proto/dto"
	"github.com/Kilat-Pet-Delivery/lib-proto/events"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/application"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestShopFulfillmentFlowPublishesDeliveredEvent(t *testing.T) {
	infra := setupContainers(t)
	defer infra.Cleanup()
	stack := setupBookingStack(t, infra.DB, infra.KafkaBrokers)
	defer stack.CleanupProducer()

	ctx := context.Background()
	ownerID := uuid.New()
	shopID := uuid.New()
	runnerID := uuid.New()
	productID := uuid.New()

	created, err := stack.Service.CreateBooking(ctx, ownerID, application.CreateBookingRequest{
		PetSpec: dto.PetSpecDTO{PetType: "cat", Name: "Milo", WeightKg: 4},
		PickupAddress: dto.AddressDTO{
			Line1: "Shop", City: "KL", State: "WP", Country: "MY", Latitude: 3.139, Longitude: 101.6869,
		},
		DropoffAddress: dto.AddressDTO{
			Line1: "Home", City: "KL", State: "WP", Country: "MY", Latitude: 3.15, Longitude: 101.71,
		},
		ShopID: &shopID,
		Items: []application.BookingItemRequest{{
			ProductID: productID, Qty: 2, PriceMyrCents: 1250, SKU: "CAT-FOOD", Name: "Cat Food",
		}},
	})
	require.NoError(t, err)

	accepted, err := stack.Service.AcceptByShop(ctx, created.ID, shopID, uuid.New(), "accept-key")
	require.NoError(t, err)
	require.Equal(t, "accepted_by_shop", accepted.Status)

	preparing, err := stack.Service.MarkPreparing(ctx, created.ID, shopID, uuid.New(), "preparing-key")
	require.NoError(t, err)
	require.Equal(t, "preparing", preparing.Status)

	ready, err := stack.Service.MarkReadyForPickup(ctx, created.ID, shopID, "ready-key")
	require.NoError(t, err)
	require.Equal(t, "ready_for_pickup", ready.Status)
	require.NotNil(t, ready.QRPickupToken)

	assigned, err := stack.Service.AcceptBooking(ctx, created.ID, runnerID)
	require.NoError(t, err)
	require.Equal(t, "accepted", assigned.Status)

	pickedUp, err := stack.Service.VerifyPickup(ctx, created.ID, runnerID, *ready.QRPickupToken, "pickup-key")
	require.NoError(t, err)
	require.Equal(t, "in_progress", pickedUp.Status)

	delivered, err := stack.Service.ConfirmDelivery(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "delivered", delivered.Status)

	cloudEvent := consumeOneEvent(t, infra.KafkaBrokers, events.TopicBookingEvents, application.BookingDelivered, 15*time.Second)
	var payload struct {
		BookingID       uuid.UUID `json:"booking_id"`
		ShopID          uuid.UUID `json:"shop_id"`
		GrossSalesCents int64     `json:"gross_sales_cents"`
		Items           []struct {
			ProductID     uuid.UUID `json:"product_id"`
			Qty           int64     `json:"qty"`
			PriceMyrCents int64     `json:"price_myr_cents"`
		} `json:"items"`
	}
	require.NoError(t, cloudEvent.ParseData(&payload))
	require.Equal(t, created.ID, payload.BookingID)
	require.Equal(t, shopID, payload.ShopID)
	require.Equal(t, int64(2500), payload.GrossSalesCents)
	require.Len(t, payload.Items, 1)
}

func TestShopAcceptRaceReturnsOneSuccessAndOneConflict(t *testing.T) {
	infra := setupContainers(t)
	defer infra.Cleanup()
	stack := setupBookingStack(t, infra.DB, infra.KafkaBrokers)
	defer stack.CleanupProducer()

	ctx := context.Background()
	shopID := uuid.New()
	created, err := stack.Service.CreateBooking(ctx, uuid.New(), application.CreateBookingRequest{
		PetSpec: dto.PetSpecDTO{PetType: "dog", Name: "Bolt", WeightKg: 8},
		PickupAddress: dto.AddressDTO{
			Line1: "Shop", City: "KL", State: "WP", Country: "MY", Latitude: 3.139, Longitude: 101.6869,
		},
		DropoffAddress: dto.AddressDTO{
			Line1: "Home", City: "KL", State: "WP", Country: "MY", Latitude: 3.15, Longitude: 101.71,
		},
		ShopID: &shopID,
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := stack.Service.AcceptByShop(ctx, created.ID, shopID, uuid.New(), uuid.NewString())
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	var successes, conflicts int
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		conflicts++
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}
