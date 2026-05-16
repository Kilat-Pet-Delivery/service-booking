//go:build integration

package main_test

import (
	"context"
	"testing"

	"github.com/Kilat-Pet-Delivery/service-booking/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeclineReasonRepository_RecordDecline verifies that RecordDecline persists
// a row in booking_decline_reasons with the correct fields.
func TestDeclineReasonRepository_RecordDecline(t *testing.T) {
	infra := setupContainers(t)
	defer infra.Cleanup()

	require.NoError(t, infra.DB.AutoMigrate(&repository.DeclineReasonModel{}))

	ownerID := uuid.New()
	runnerID := uuid.New()
	bookingID := uuid.New()

	// Seed a booking row so the FK constraint is satisfied.
	seedAcceptedBooking(t, infra.DB, bookingID, ownerID, runnerID)

	repo := repository.NewGormDeclineReasonRepository(infra.DB)
	err := repo.RecordDecline(context.Background(), infra.DB, bookingID, runnerID, "already_busy")
	require.NoError(t, err)

	var row repository.DeclineReasonModel
	err = infra.DB.Where("booking_id = ? AND runner_id = ?", bookingID, runnerID).First(&row).Error
	require.NoError(t, err)
	assert.Equal(t, bookingID, row.BookingID)
	assert.Equal(t, runnerID, row.RunnerID)
	assert.Equal(t, "already_busy", row.Reason)
	assert.False(t, row.DeclinedAt.IsZero())
}
