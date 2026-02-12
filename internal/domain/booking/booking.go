package booking

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/domain"
	"github.com/Kilat-Pet-Delivery/lib-proto/dto"
	"github.com/google/uuid"
)

const bookingNumberChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Booking is the aggregate root for the booking domain.
type Booking struct {
	id             uuid.UUID
	bookingNumber  string
	ownerID        uuid.UUID
	runnerID       *uuid.UUID
	status         BookingStatus
	petSpec        PetSpecification
	crateReq       CrateRequirement
	pickupAddress  dto.AddressDTO
	dropoffAddress dto.AddressDTO
	routeSpec      *RouteSpecification

	estimatedPriceCents int64
	finalPriceCents     *int64
	currency            string

	scheduledAt *time.Time
	pickedUpAt  *time.Time
	deliveredAt *time.Time
	cancelledAt *time.Time
	cancelNote  string
	notes       string

	version   int64
	createdAt time.Time
	updatedAt time.Time
}

// generateBookingNumber creates a booking number in the format "BK-XXXXXX".
func generateBookingNumber() (string, error) {
	result := make([]byte, 6)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(bookingNumberChars))))
		if err != nil {
			return "", fmt.Errorf("failed to generate booking number: %w", err)
		}
		result[i] = bookingNumberChars[n.Int64()]
	}
	return "BK-" + string(result), nil
}

// NewBooking creates a new Booking aggregate with status=requested.
func NewBooking(
	ownerID uuid.UUID,
	petSpec PetSpecification,
	crateReq CrateRequirement,
	pickupAddress dto.AddressDTO,
	dropoffAddress dto.AddressDTO,
	estimatedPriceCents int64,
	currency string,
	scheduledAt *time.Time,
	notes string,
) (*Booking, error) {
	if ownerID == uuid.Nil {
		return nil, domain.NewValidationError("owner ID is required")
	}
	if petSpec.Name == "" {
		return nil, domain.NewValidationError("pet name is required")
	}
	if !PetType(petSpec.PetType).IsValid() {
		return nil, domain.NewValidationError(fmt.Sprintf("invalid pet type: %s", petSpec.PetType))
	}
	if petSpec.WeightKg <= 0 {
		return nil, domain.NewValidationError("pet weight must be positive")
	}
	if pickupAddress.Line1 == "" {
		return nil, domain.NewValidationError("pickup address is required")
	}
	if dropoffAddress.Line1 == "" {
		return nil, domain.NewValidationError("dropoff address is required")
	}
	if estimatedPriceCents <= 0 {
		return nil, domain.NewValidationError("estimated price must be positive")
	}

	bookingNumber, err := generateBookingNumber()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &Booking{
		id:                  uuid.New(),
		bookingNumber:       bookingNumber,
		ownerID:             ownerID,
		status:              StatusRequested,
		petSpec:             petSpec,
		crateReq:            crateReq,
		pickupAddress:       pickupAddress,
		dropoffAddress:      dropoffAddress,
		estimatedPriceCents: estimatedPriceCents,
		currency:            currency,
		scheduledAt:         scheduledAt,
		notes:               notes,
		version:             1,
		createdAt:           now,
		updatedAt:           now,
	}, nil
}

// ReconstructBooking rebuilds a Booking from persistence data (no validation).
func ReconstructBooking(
	id uuid.UUID,
	bookingNumber string,
	ownerID uuid.UUID,
	runnerID *uuid.UUID,
	status BookingStatus,
	petSpec PetSpecification,
	crateReq CrateRequirement,
	pickupAddress dto.AddressDTO,
	dropoffAddress dto.AddressDTO,
	routeSpec *RouteSpecification,
	estimatedPriceCents int64,
	finalPriceCents *int64,
	currency string,
	scheduledAt *time.Time,
	pickedUpAt *time.Time,
	deliveredAt *time.Time,
	cancelledAt *time.Time,
	cancelNote string,
	notes string,
	version int64,
	createdAt time.Time,
	updatedAt time.Time,
) *Booking {
	return &Booking{
		id:                  id,
		bookingNumber:       bookingNumber,
		ownerID:             ownerID,
		runnerID:            runnerID,
		status:              status,
		petSpec:             petSpec,
		crateReq:            crateReq,
		pickupAddress:       pickupAddress,
		dropoffAddress:      dropoffAddress,
		routeSpec:           routeSpec,
		estimatedPriceCents: estimatedPriceCents,
		finalPriceCents:     finalPriceCents,
		currency:            currency,
		scheduledAt:         scheduledAt,
		pickedUpAt:          pickedUpAt,
		deliveredAt:         deliveredAt,
		cancelledAt:         cancelledAt,
		cancelNote:          cancelNote,
		notes:               notes,
		version:             version,
		createdAt:           createdAt,
		updatedAt:           updatedAt,
	}
}

// --- Getters ---

// ID returns the booking's unique identifier.
func (b *Booking) ID() uuid.UUID { return b.id }

// BookingNumber returns the human-readable booking number.
func (b *Booking) BookingNumber() string { return b.bookingNumber }

// OwnerID returns the pet owner's user ID.
func (b *Booking) OwnerID() uuid.UUID { return b.ownerID }

// RunnerID returns the assigned runner's user ID, or nil if unassigned.
func (b *Booking) RunnerID() *uuid.UUID { return b.runnerID }

// Status returns the current booking status.
func (b *Booking) Status() BookingStatus { return b.status }

// PetSpec returns the pet specification.
func (b *Booking) PetSpec() PetSpecification { return b.petSpec }

// CrateReq returns the crate requirement.
func (b *Booking) CrateReq() CrateRequirement { return b.crateReq }

// PickupAddress returns the pickup address.
func (b *Booking) PickupAddress() dto.AddressDTO { return b.pickupAddress }

// DropoffAddress returns the dropoff address.
func (b *Booking) DropoffAddress() dto.AddressDTO { return b.dropoffAddress }

// RouteSpec returns the route specification, or nil if not yet calculated.
func (b *Booking) RouteSpec() *RouteSpecification { return b.routeSpec }

// EstimatedPriceCents returns the estimated price in cents.
func (b *Booking) EstimatedPriceCents() int64 { return b.estimatedPriceCents }

// FinalPriceCents returns the final price in cents, or nil if not yet finalized.
func (b *Booking) FinalPriceCents() *int64 { return b.finalPriceCents }

// Currency returns the currency code.
func (b *Booking) Currency() string { return b.currency }

// ScheduledAt returns the scheduled time, or nil if immediate.
func (b *Booking) ScheduledAt() *time.Time { return b.scheduledAt }

// PickedUpAt returns the time the pet was picked up.
func (b *Booking) PickedUpAt() *time.Time { return b.pickedUpAt }

// DeliveredAt returns the time the pet was delivered.
func (b *Booking) DeliveredAt() *time.Time { return b.deliveredAt }

// CancelledAt returns the time the booking was cancelled.
func (b *Booking) CancelledAt() *time.Time { return b.cancelledAt }

// CancelNote returns the cancellation reason.
func (b *Booking) CancelNote() string { return b.cancelNote }

// Notes returns any additional notes for the booking.
func (b *Booking) Notes() string { return b.notes }

// Version returns the entity version for optimistic locking.
func (b *Booking) Version() int64 { return b.version }

// CreatedAt returns the creation timestamp.
func (b *Booking) CreatedAt() time.Time { return b.createdAt }

// UpdatedAt returns the last-updated timestamp.
func (b *Booking) UpdatedAt() time.Time { return b.updatedAt }

// --- Behavior ---

// Accept transitions the booking from requested to accepted with the given runner.
func (b *Booking) Accept(runnerID uuid.UUID) error {
	if !b.status.CanTransitionTo(StatusAccepted) {
		return domain.NewInvalidStateError(string(b.status), string(StatusAccepted))
	}
	if runnerID == uuid.Nil {
		return domain.NewValidationError("runner ID is required")
	}
	b.runnerID = &runnerID
	b.status = StatusAccepted
	b.updatedAt = time.Now().UTC()
	return nil
}

// StartDelivery transitions the booking from accepted to in_progress.
func (b *Booking) StartDelivery() error {
	if !b.status.CanTransitionTo(StatusInProgress) {
		return domain.NewInvalidStateError(string(b.status), string(StatusInProgress))
	}
	now := time.Now().UTC()
	b.status = StatusInProgress
	b.pickedUpAt = &now
	b.updatedAt = now
	return nil
}

// ConfirmDelivery transitions the booking from in_progress to delivered.
func (b *Booking) ConfirmDelivery() error {
	if !b.status.CanTransitionTo(StatusDelivered) {
		return domain.NewInvalidStateError(string(b.status), string(StatusDelivered))
	}
	now := time.Now().UTC()
	b.status = StatusDelivered
	b.deliveredAt = &now
	b.updatedAt = now
	return nil
}

// Complete transitions the booking from delivered to completed with the final price.
func (b *Booking) Complete(finalPriceCents int64) error {
	if !b.status.CanTransitionTo(StatusCompleted) {
		return domain.NewInvalidStateError(string(b.status), string(StatusCompleted))
	}
	b.status = StatusCompleted
	b.finalPriceCents = &finalPriceCents
	b.updatedAt = time.Now().UTC()
	return nil
}

// Cancel transitions the booking to cancelled if it is not in a terminal state.
func (b *Booking) Cancel(reason string) error {
	if !b.status.CanBeCancelled() {
		return domain.NewInvalidStateError(string(b.status), string(StatusCancelled))
	}
	now := time.Now().UTC()
	b.status = StatusCancelled
	b.cancelNote = reason
	b.cancelledAt = &now
	b.updatedAt = now
	return nil
}

// IncrementVersion bumps the version for optimistic locking.
func (b *Booking) IncrementVersion() {
	b.version++
	b.updatedAt = time.Now().UTC()
}

// SetRouteSpec sets the route specification for this booking.
func (b *Booking) SetRouteSpec(routeSpec *RouteSpecification) {
	b.routeSpec = routeSpec
	b.updatedAt = time.Now().UTC()
}
