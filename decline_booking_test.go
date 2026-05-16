//go:build integration

package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/auth"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/application"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/handler"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/repository"
	bookingDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/booking"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// --- Test setup helpers ---

// declineTestStack builds a minimal HTTP test server wired to real Postgres.
type declineTestStack struct {
	Router     *gin.Engine
	JWTManager *auth.JWTManager
	DB         *gorm.DB
}

func setupDeclineStack(t *testing.T, db *gorm.DB) *declineTestStack {
	t.Helper()

	// Auto-migrate the decline reasons table for tests.
	require.NoError(t, db.AutoMigrate(&repository.DeclineReasonModel{}))

	logger, _ := zap.NewDevelopment()
	jwtManager := auth.NewJWTManager("test-secret-key", 15*time.Minute, 24*time.Hour)

	bookingRepo := repository.NewGormBookingRepository(db)
	declineRepo := repository.NewGormDeclineReasonRepository(db)
	pricing := bookingDomain.NewStandardPricingStrategy()

	// Use a no-op Kafka producer (nil) — decline doesn't publish events.
	svc := application.NewBookingService(bookingRepo, pricing, nil, logger).
		WithDeclineSupport(db, declineRepo)

	bookingHandler := handler.NewBookingHandler(svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	rg := &router.RouterGroup
	bookingHandler.RegisterRoutes(rg, jwtManager)

	return &declineTestStack{
		Router:     router,
		JWTManager: jwtManager,
		DB:         db,
	}
}

// runnerToken generates a valid JWT for a runner.
func runnerToken(t *testing.T, jwtManager *auth.JWTManager, runnerID uuid.UUID) string {
	t.Helper()
	token, err := jwtManager.GenerateAccessToken(runnerID, "runner@test.com", auth.RoleRunner)
	require.NoError(t, err)
	return token
}

// seedAcceptedBooking inserts a booking in "accepted" state with the given runnerID.
func seedAcceptedBooking(t *testing.T, db *gorm.DB, bookingID, ownerID, runnerID uuid.UUID) {
	t.Helper()
	petSpec, _ := json.Marshal(map[string]interface{}{
		"pet_type": "cat", "name": "Whiskers", "weight_kg": 4.0,
	})
	crateReq, _ := json.Marshal(map[string]interface{}{
		"minimum_size": "small", "needs_ventilation": true, "minimum_weight_capacity": 4.8,
	})
	pickup, _ := json.Marshal(map[string]interface{}{
		"line1": "1 Pickup St", "city": "KL", "state": "WP",
		"country": "MY", "latitude": 3.139, "longitude": 101.6869,
	})
	dropoff, _ := json.Marshal(map[string]interface{}{
		"line1": "2 Dropoff Ave", "city": "KL", "state": "WP",
		"country": "MY", "latitude": 3.15, "longitude": 101.71,
	})
	now := time.Now().UTC()
	model := repository.BookingModel{
		ID:                  bookingID,
		BookingNumber:       fmt.Sprintf("BK-DEC%s", uuid.New().String()[:6]),
		OwnerID:             ownerID,
		RunnerID:            &runnerID,
		Status:              "accepted",
		PetSpec:             petSpec,
		CrateRequirement:    crateReq,
		PickupAddress:       pickup,
		DropoffAddress:      dropoff,
		EstimatedPriceCents: 100000,
		Currency:            "MYR",
		Notes:               "decline integration test",
		Version:             2,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	require.NoError(t, db.Create(&model).Error)
}

// seedCompletedBooking inserts a booking in "completed" state.
func seedCompletedBooking(t *testing.T, db *gorm.DB, bookingID, ownerID, runnerID uuid.UUID) {
	t.Helper()
	petSpec, _ := json.Marshal(map[string]interface{}{
		"pet_type": "dog", "name": "Rex", "weight_kg": 10.0,
	})
	crateReq, _ := json.Marshal(map[string]interface{}{
		"minimum_size": "medium", "needs_ventilation": true, "minimum_weight_capacity": 12.0,
	})
	pickup, _ := json.Marshal(map[string]interface{}{
		"line1": "3 Pickup Rd", "city": "KL", "state": "WP",
		"country": "MY", "latitude": 3.14, "longitude": 101.69,
	})
	dropoff, _ := json.Marshal(map[string]interface{}{
		"line1": "4 Dropoff Rd", "city": "KL", "state": "WP",
		"country": "MY", "latitude": 3.16, "longitude": 101.72,
	})
	now := time.Now().UTC()
	finalPrice := int64(200000)
	model := repository.BookingModel{
		ID:                  bookingID,
		BookingNumber:       fmt.Sprintf("BK-CMP%s", uuid.New().String()[:6]),
		OwnerID:             ownerID,
		RunnerID:            &runnerID,
		Status:              "completed",
		PetSpec:             petSpec,
		CrateRequirement:    crateReq,
		PickupAddress:       pickup,
		DropoffAddress:      dropoff,
		EstimatedPriceCents: 200000,
		FinalPriceCents:     &finalPrice,
		Currency:            "MYR",
		Notes:               "completed booking test",
		Version:             5,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	require.NoError(t, db.Create(&model).Error)
}

// doDeclineRequest fires a POST decline request against the test router.
func doDeclineRequest(t *testing.T, router *gin.Engine, bookingID uuid.UUID, token, reason string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"reason": reason})
	req := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/v1/bookings/%s/decline", bookingID),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- Tests ---

// TestDeclineBooking_ValidReason_Returns200_PersistsReason_TransitionsBooking verifies the
// happy path: runner declines their accepted booking with a valid reason. Expects HTTP 200,
// booking status transitions to "requested", runner_id cleared, and a row in booking_decline_reasons.
func TestDeclineBooking_ValidReason_Returns200_PersistsReason_TransitionsBooking(t *testing.T) {
	infra := setupContainers(t)
	defer infra.Cleanup()

	stack := setupDeclineStack(t, infra.DB)

	bookingID := uuid.New()
	ownerID := uuid.New()
	runnerID := uuid.New()
	seedAcceptedBooking(t, infra.DB, bookingID, ownerID, runnerID)

	token := runnerToken(t, stack.JWTManager, runnerID)
	w := doDeclineRequest(t, stack.Router, bookingID, token, "too_far")

	assert.Equal(t, http.StatusOK, w.Code, "expected 200, got: %s", w.Body.String())

	// Assert booking status is now "requested" and runner_id is NULL.
	var bm repository.BookingModel
	require.NoError(t, infra.DB.WithContext(context.Background()).
		Where("id = ?", bookingID).First(&bm).Error)
	assert.Equal(t, "requested", bm.Status)
	assert.Nil(t, bm.RunnerID, "runner_id should be cleared after decline")

	// Assert a decline reason row was persisted.
	var declineRow repository.DeclineReasonModel
	err := infra.DB.WithContext(context.Background()).
		Where("booking_id = ? AND runner_id = ?", bookingID, runnerID).
		First(&declineRow).Error
	require.NoError(t, err, "expected a row in booking_decline_reasons")
	assert.Equal(t, "too_far", declineRow.Reason)
}

// TestDeclineBooking_NotRunnersBooking_Returns403 verifies that a runner who does not own
// the booking receives a 403 Forbidden response.
func TestDeclineBooking_NotRunnersBooking_Returns403(t *testing.T) {
	infra := setupContainers(t)
	defer infra.Cleanup()

	stack := setupDeclineStack(t, infra.DB)

	bookingID := uuid.New()
	ownerID := uuid.New()
	actualRunnerID := uuid.New()
	otherRunnerID := uuid.New() // this runner does NOT own the booking
	seedAcceptedBooking(t, infra.DB, bookingID, ownerID, actualRunnerID)

	token := runnerToken(t, stack.JWTManager, otherRunnerID)
	w := doDeclineRequest(t, stack.Router, bookingID, token, "too_far")

	assert.Equal(t, http.StatusForbidden, w.Code, "expected 403, got: %s", w.Body.String())
}

// TestDeclineBooking_InvalidReason_Returns400 verifies that an unrecognised decline reason
// is rejected with a 400 Bad Request before reaching the service layer.
func TestDeclineBooking_InvalidReason_Returns400(t *testing.T) {
	infra := setupContainers(t)
	defer infra.Cleanup()

	stack := setupDeclineStack(t, infra.DB)

	bookingID := uuid.New()
	ownerID := uuid.New()
	runnerID := uuid.New()
	seedAcceptedBooking(t, infra.DB, bookingID, ownerID, runnerID)

	token := runnerToken(t, stack.JWTManager, runnerID)
	w := doDeclineRequest(t, stack.Router, bookingID, token, "not_a_valid_reason")

	assert.Equal(t, http.StatusBadRequest, w.Code, "expected 400, got: %s", w.Body.String())
}

// TestDeclineBooking_AlreadyCompleted_Returns409 verifies that attempting to decline a
// booking that is already in "completed" state returns 409 Conflict.
func TestDeclineBooking_AlreadyCompleted_Returns409(t *testing.T) {
	infra := setupContainers(t)
	defer infra.Cleanup()

	stack := setupDeclineStack(t, infra.DB)

	bookingID := uuid.New()
	ownerID := uuid.New()
	runnerID := uuid.New()
	seedCompletedBooking(t, infra.DB, bookingID, ownerID, runnerID)

	token := runnerToken(t, stack.JWTManager, runnerID)
	w := doDeclineRequest(t, stack.Router, bookingID, token, "too_far")

	assert.Equal(t, http.StatusConflict, w.Code, "expected 409, got: %s", w.Body.String())
}

