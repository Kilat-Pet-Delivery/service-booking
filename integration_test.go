//go:build integration

package main_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-proto/events"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEscrowReleased_CompletesBooking verifies that when an EscrowReleasedEvent
// is published to payment.events, the booking service picks it up and
// transitions the booking to "completed" status.
func TestEscrowReleased_CompletesBooking(t *testing.T) {
	infra := setupContainers(t)
	defer infra.Cleanup()

	stack := setupBookingStack(t, infra.DB, infra.KafkaBrokers)
	defer stack.CleanupProducer()
	defer func() { _ = stack.Consumer.Close() }()

	// Seed a booking in "delivered" state.
	bookingID := uuid.New()
	ownerID := uuid.New()
	runnerID := uuid.New()
	seedBookingInDeliveredState(t, infra.DB, bookingID, ownerID, runnerID)

	// Start the consumer.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = stack.Consumer.Start(ctx) }()
	time.Sleep(3 * time.Second) // Wait for consumer group join.

	// Publish EscrowReleasedEvent.
	evt := events.EscrowReleasedEvent{
		PaymentID:    uuid.New(),
		BookingID:    bookingID,
		RunnerID:     runnerID,
		RunnerPayout: 127500,
		PlatformFee:  22500,
		Currency:     "MYR",
		OccurredAt:   time.Now().UTC(),
	}
	publishTestEvent(t, infra.KafkaBrokers, events.TopicPaymentEvents,
		"service-payment", events.PaymentEscrowReleased, evt)

	// Assert: booking transitions to "completed".
	model := waitForBookingStatus(t, infra.DB, bookingID, "completed", 15*time.Second)
	assert.NotNil(t, model.FinalPriceCents, "final_price_cents should be set")
	assert.Equal(t, int64(150000), *model.FinalPriceCents)

	// Assert: BookingCompletedEvent on booking.events.
	ce := consumeOneEvent(t, infra.KafkaBrokers, events.TopicBookingEvents,
		events.BookingCompleted, 15*time.Second)

	var completed events.BookingCompletedEvent
	require.NoError(t, ce.ParseData(&completed))
	assert.Equal(t, bookingID, completed.BookingID)
	assert.Equal(t, runnerID, completed.RunnerID)
	assert.Equal(t, int64(150000), completed.FinalPrice)
	assert.Equal(t, "MYR", completed.Currency)
}
