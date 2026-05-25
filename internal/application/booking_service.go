package application

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/domain"
	"github.com/Kilat-Pet-Delivery/lib-common/kafka"
	"github.com/Kilat-Pet-Delivery/lib-proto/dto"
	"github.com/Kilat-Pet-Delivery/lib-proto/events"
	bookingDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/booking"
	photoDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/photo"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const authRoleRunner = "runner"

const (
	BookingArrivedAtPickup = "booking.arrived_at_pickup"
	BookingProofSubmitted  = "booking.proof_submitted"
	BookingCancelledActive = "booking.cancelled_active"
)

// CreateBookingRequest holds the data needed to create a new booking.
type CreateBookingRequest struct {
	PetSpec        dto.PetSpecDTO `json:"pet_spec" binding:"required"`
	PickupAddress  dto.AddressDTO `json:"pickup_address" binding:"required"`
	DropoffAddress dto.AddressDTO `json:"dropoff_address" binding:"required"`
	ScheduledAt    *time.Time     `json:"scheduled_at"`
	Notes          string         `json:"notes"`
}

// BookingDTO is the response representation of a booking.
type BookingDTO struct {
	ID                  uuid.UUID                         `json:"id"`
	BookingNumber       string                            `json:"booking_number"`
	OwnerID             uuid.UUID                         `json:"owner_id"`
	RunnerID            *uuid.UUID                        `json:"runner_id,omitempty"`
	Status              string                            `json:"status"`
	PetSpec             bookingDomain.PetSpecification    `json:"pet_spec"`
	CrateReq            bookingDomain.CrateRequirement    `json:"crate_requirement"`
	PickupAddress       dto.AddressDTO                    `json:"pickup_address"`
	DropoffAddress      dto.AddressDTO                    `json:"dropoff_address"`
	RouteSpec           *bookingDomain.RouteSpecification `json:"route_spec,omitempty"`
	EstimatedPriceCents int64                             `json:"estimated_price_cents"`
	FinalPriceCents     *int64                            `json:"final_price_cents,omitempty"`
	Currency            string                            `json:"currency"`
	ScheduledAt         *time.Time                        `json:"scheduled_at,omitempty"`
	ArrivedAtPickup     *time.Time                        `json:"arrived_at_pickup,omitempty"`
	PickedUpAt          *time.Time                        `json:"picked_up_at,omitempty"`
	DeliveredAt         *time.Time                        `json:"delivered_at,omitempty"`
	CancelledAt         *time.Time                        `json:"cancelled_at,omitempty"`
	CancelNote          string                            `json:"cancel_note,omitempty"`
	Notes               string                            `json:"notes,omitempty"`
	Version             int64                             `json:"version"`
	CreatedAt           time.Time                         `json:"created_at"`
	UpdatedAt           time.Time                         `json:"updated_at"`
}

// ProofOfDeliveryRequest holds final delivery handoff evidence.
type ProofOfDeliveryRequest struct {
	PhotoURL      string `json:"photo_url" form:"photo_url" binding:"required"`
	SignatureURL  string `json:"signature_url" form:"signature_url" binding:"required"`
	RecipientKind string `json:"recipient_kind" form:"recipient_kind" binding:"required"`
	Notes         string `json:"notes" form:"notes"`
}

// ProofOfDeliveryDTO is the API response for delivery proof evidence.
type ProofOfDeliveryDTO struct {
	ID            uuid.UUID `json:"id"`
	BookingID     uuid.UUID `json:"booking_id"`
	PhotoURL      string    `json:"photo_url"`
	SignatureURL  string    `json:"signature_url"`
	RecipientKind string    `json:"recipient_kind"`
	Notes         string    `json:"notes,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type BookingArrivedAtPickupEvent struct {
	BookingID       uuid.UUID `json:"booking_id"`
	BookingNumber   string    `json:"booking_number"`
	RunnerID        uuid.UUID `json:"runner_id"`
	OwnerID         uuid.UUID `json:"owner_id"`
	ArrivedAtPickup time.Time `json:"arrived_at_pickup"`
	OccurredAt      time.Time `json:"occurred_at"`
}

type BookingProofSubmittedEvent struct {
	BookingID     uuid.UUID `json:"booking_id"`
	BookingNumber string    `json:"booking_number"`
	RunnerID      uuid.UUID `json:"runner_id"`
	OwnerID       uuid.UUID `json:"owner_id"`
	ProofID       uuid.UUID `json:"proof_id"`
	RecipientKind string    `json:"recipient_kind"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type BookingCancelledActiveEvent struct {
	BookingID     uuid.UUID `json:"booking_id"`
	BookingNumber string    `json:"booking_number"`
	CancelledBy   uuid.UUID `json:"cancelled_by"`
	Reason        string    `json:"reason"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// BookingService is the application service orchestrating booking use cases.
type BookingService struct {
	repo        bookingDomain.BookingRepository
	proofRepo   bookingDomain.ProofOfDeliveryRepository
	photoRepo   photoDomain.PhotoRepository
	declineRepo *repository.GormDeclineReasonRepository
	db          *gorm.DB
	pricing     bookingDomain.PricingStrategy
	producer    *kafka.Producer
	logger      *zap.Logger
}

// NewBookingService creates a new BookingService.
func NewBookingService(
	repo bookingDomain.BookingRepository,
	pricing bookingDomain.PricingStrategy,
	producer *kafka.Producer,
	logger *zap.Logger,
	db *gorm.DB,
	declineRepo *repository.GormDeclineReasonRepository,
) *BookingService {
	var proofRepo bookingDomain.ProofOfDeliveryRepository
	var photoRepo photoDomain.PhotoRepository
	if db != nil {
		proofRepo = repository.NewGormProofOfDeliveryRepository(db)
		photoRepo = repository.NewGormPhotoRepository(db)
	}
	return &BookingService{
		repo:        repo,
		proofRepo:   proofRepo,
		photoRepo:   photoRepo,
		pricing:     pricing,
		producer:    producer,
		logger:      logger,
		db:          db,
		declineRepo: declineRepo,
	}
}

// CreateBooking creates a new booking for the given owner.
func (s *BookingService) CreateBooking(ctx context.Context, ownerID uuid.UUID, req CreateBookingRequest) (*BookingDTO, error) {
	// Build pet specification from DTO
	petSpec := buildPetSpecification(req.PetSpec)

	// Determine crate requirement
	crateReq := bookingDomain.DetermineCrateRequirement(petSpec)

	// Calculate distance (Haversine approximation)
	distanceKm := haversineDistance(
		req.PickupAddress.Latitude, req.PickupAddress.Longitude,
		req.DropoffAddress.Latitude, req.DropoffAddress.Longitude,
	)

	// Calculate estimated price
	priceCents, err := s.pricing.Calculate(bookingDomain.PricingParams{
		DistanceKm:  distanceKm,
		PetType:     bookingDomain.PetType(req.PetSpec.PetType),
		CrateSize:   crateReq.MinimumSize,
		IsScheduled: req.ScheduledAt != nil,
	})
	if err != nil {
		return nil, domain.NewValidationError(fmt.Sprintf("pricing error: %v", err))
	}

	// Create the booking aggregate
	bk, err := bookingDomain.NewBooking(
		ownerID,
		petSpec,
		crateReq,
		req.PickupAddress,
		req.DropoffAddress,
		priceCents,
		domain.CurrencyMYR,
		req.ScheduledAt,
		req.Notes,
	)
	if err != nil {
		return nil, err
	}

	// Persist the booking
	if err := s.repo.Save(ctx, bk); err != nil {
		return nil, fmt.Errorf("failed to save booking: %w", err)
	}

	// Publish BookingRequestedEvent
	s.publishBookingRequested(ctx, bk)

	result := toBookingDTO(bk)
	return &result, nil
}

// AcceptBooking assigns a runner to an open booking.
func (s *BookingService) AcceptBooking(ctx context.Context, bookingID, runnerID uuid.UUID) (*BookingDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	if err := bk.Accept(runnerID); err != nil {
		return nil, err
	}

	bk.IncrementVersion()
	if err := s.repo.Update(ctx, bk); err != nil {
		return nil, err
	}

	// Publish BookingAcceptedEvent
	evt := events.BookingAcceptedEvent{
		BookingID:     bk.ID(),
		BookingNumber: bk.BookingNumber(),
		RunnerID:      runnerID,
		OwnerID:       bk.OwnerID(),
		OccurredAt:    time.Now().UTC(),
	}
	s.publishEvent(ctx, events.TopicBookingEvents, events.BookingAccepted, bk.ID().String(), evt)

	result := toBookingDTO(bk)
	return &result, nil
}

// ArriveAtPickup records runner arrival at the pickup location.
func (s *BookingService) ArriveAtPickup(ctx context.Context, bookingID uuid.UUID) (*BookingDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	if err := bk.ArriveAtPickup(); err != nil {
		return nil, err
	}

	bk.IncrementVersion()
	if err := s.repo.Update(ctx, bk); err != nil {
		return nil, err
	}

	evt := BookingArrivedAtPickupEvent{
		BookingID:       bk.ID(),
		BookingNumber:   bk.BookingNumber(),
		RunnerID:        derefUUID(bk.RunnerID()),
		OwnerID:         bk.OwnerID(),
		ArrivedAtPickup: *bk.ArrivedAtPickup(),
		OccurredAt:      time.Now().UTC(),
	}
	s.publishEvent(ctx, events.TopicBookingEvents, BookingArrivedAtPickup, bk.ID().String(), evt)

	result := toBookingDTO(bk)
	return &result, nil
}

// StartDelivery marks the pet as picked up and delivery in progress.
func (s *BookingService) StartDelivery(ctx context.Context, bookingID uuid.UUID) (*BookingDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	if err := bk.StartDelivery(); err != nil {
		return nil, err
	}

	bk.IncrementVersion()
	if err := s.repo.Update(ctx, bk); err != nil {
		return nil, err
	}

	// Publish PetPickedUpEvent
	evt := events.PetPickedUpEvent{
		BookingID:     bk.ID(),
		BookingNumber: bk.BookingNumber(),
		RunnerID:      *bk.RunnerID(),
		OwnerID:       bk.OwnerID(),
		PickedUpAt:    *bk.PickedUpAt(),
		OccurredAt:    time.Now().UTC(),
	}
	s.publishEvent(ctx, events.TopicBookingEvents, events.BookingPetPickedUp, bk.ID().String(), evt)

	result := toBookingDTO(bk)
	return &result, nil
}

// SubmitProofOfDelivery persists delivery proof and publishes a proof-submitted event.
func (s *BookingService) SubmitProofOfDelivery(ctx context.Context, bookingID, runnerID uuid.UUID, req ProofOfDeliveryRequest) (*ProofOfDeliveryDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if bk.Status() != bookingDomain.StatusDelivered {
		return nil, domain.NewInvalidStateError(string(bk.Status()), "proof_of_delivery")
	}
	if s.proofRepo == nil {
		return nil, domain.NewValidationError("proof repository is not configured")
	}

	proof, err := bookingDomain.NewProofOfDelivery(bookingID, req.PhotoURL, req.SignatureURL, req.RecipientKind, req.Notes)
	if err != nil {
		return nil, domain.NewValidationError(err.Error())
	}
	if err := s.proofRepo.Save(ctx, proof); err != nil {
		return nil, err
	}
	if s.photoRepo != nil {
		photo, err := photoDomain.NewBookingPhoto(bookingID, runnerID, photoDomain.PhotoTypeProof, req.PhotoURL, "proof of delivery")
		if err != nil {
			return nil, err
		}
		if err := s.photoRepo.Save(ctx, photo); err != nil {
			return nil, err
		}
		signature, err := photoDomain.NewBookingPhoto(bookingID, runnerID, photoDomain.PhotoTypeSignature, req.SignatureURL, "recipient signature")
		if err != nil {
			return nil, err
		}
		if err := s.photoRepo.Save(ctx, signature); err != nil {
			return nil, err
		}
	}

	evt := BookingProofSubmittedEvent{
		BookingID:     bk.ID(),
		BookingNumber: bk.BookingNumber(),
		RunnerID:      runnerID,
		OwnerID:       bk.OwnerID(),
		ProofID:       proof.ID(),
		RecipientKind: proof.RecipientKind(),
		OccurredAt:    time.Now().UTC(),
	}
	s.publishEvent(ctx, events.TopicBookingEvents, BookingProofSubmitted, bk.ID().String(), evt)

	result := toProofDTO(proof)
	return &result, nil
}

// ConfirmDelivery marks the pet as delivered at the dropoff location.
func (s *BookingService) ConfirmDelivery(ctx context.Context, bookingID uuid.UUID) (*BookingDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	if err := bk.ConfirmDelivery(); err != nil {
		return nil, err
	}

	bk.IncrementVersion()
	if err := s.repo.Update(ctx, bk); err != nil {
		return nil, err
	}

	// Publish DeliveryConfirmedEvent
	evt := events.DeliveryConfirmedEvent{
		BookingID:     bk.ID(),
		BookingNumber: bk.BookingNumber(),
		RunnerID:      *bk.RunnerID(),
		OwnerID:       bk.OwnerID(),
		DeliveredAt:   *bk.DeliveredAt(),
		OccurredAt:    time.Now().UTC(),
	}
	s.publishEvent(ctx, events.TopicBookingEvents, events.BookingDeliveryConfirmed, bk.ID().String(), evt)

	result := toBookingDTO(bk)
	return &result, nil
}

// CompleteBooking finalizes the booking after payment escrow is released.
func (s *BookingService) CompleteBooking(ctx context.Context, bookingID uuid.UUID) (*BookingDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if bk.Status() == bookingDomain.StatusCompleted {
		result := toBookingDTO(bk)
		return &result, nil
	}

	// Use estimated price as final price if not set differently
	finalPrice := bk.EstimatedPriceCents()

	if err := bk.Complete(finalPrice); err != nil {
		return nil, err
	}

	bk.IncrementVersion()
	if err := s.repo.Update(ctx, bk); err != nil {
		return nil, err
	}

	// Publish BookingCompletedEvent
	var runnerID uuid.UUID
	if bk.RunnerID() != nil {
		runnerID = *bk.RunnerID()
	}
	evt := events.BookingCompletedEvent{
		BookingID:     bk.ID(),
		BookingNumber: bk.BookingNumber(),
		RunnerID:      runnerID,
		OwnerID:       bk.OwnerID(),
		FinalPrice:    finalPrice,
		Currency:      bk.Currency(),
		OccurredAt:    time.Now().UTC(),
	}
	s.publishEvent(ctx, events.TopicBookingEvents, events.BookingCompleted, bk.ID().String(), evt)

	result := toBookingDTO(bk)
	return &result, nil
}

// CompleteDelivery is the public delivery-complete use case. It is idempotent.
func (s *BookingService) CompleteDelivery(ctx context.Context, bookingID uuid.UUID) (*BookingDTO, error) {
	return s.CompleteBooking(ctx, bookingID)
}

// CancelBooking cancels a booking that is not yet in a terminal state.
func (s *BookingService) CancelBooking(ctx context.Context, bookingID, cancelledBy uuid.UUID, reason string) (*BookingDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	if err := bk.Cancel(reason); err != nil {
		return nil, err
	}

	bk.IncrementVersion()
	if err := s.repo.Update(ctx, bk); err != nil {
		return nil, err
	}

	// Publish BookingCancelledEvent
	evt := events.BookingCancelledEvent{
		BookingID:     bk.ID(),
		BookingNumber: bk.BookingNumber(),
		CancelledBy:   cancelledBy,
		Reason:        reason,
		OccurredAt:    time.Now().UTC(),
	}
	s.publishEvent(ctx, events.TopicBookingEvents, events.BookingCancelled, bk.ID().String(), evt)

	result := toBookingDTO(bk)
	return &result, nil
}

// CancelActiveDelivery cancels an active delivery and emits an event for service-incident.
func (s *BookingService) CancelActiveDelivery(ctx context.Context, bookingID, cancelledBy uuid.UUID, reason string) (*BookingDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if bk.Status() != bookingDomain.StatusPickupArrived && bk.Status() != bookingDomain.StatusInProgress {
		return nil, domain.NewInvalidStateError(string(bk.Status()), string(bookingDomain.StatusCancelled))
	}
	if reason == "" {
		return nil, domain.NewValidationError("reason is required")
	}

	if err := bk.Cancel(reason); err != nil {
		return nil, err
	}
	bk.IncrementVersion()
	if err := s.repo.Update(ctx, bk); err != nil {
		return nil, err
	}

	evt := BookingCancelledActiveEvent{
		BookingID:     bk.ID(),
		BookingNumber: bk.BookingNumber(),
		CancelledBy:   cancelledBy,
		Reason:        reason,
		OccurredAt:    time.Now().UTC(),
	}
	s.publishEvent(ctx, events.TopicBookingEvents, BookingCancelledActive, bk.ID().String(), evt)

	result := toBookingDTO(bk)
	return &result, nil
}

// GetBooking retrieves a single booking by ID.
func (s *BookingService) GetBooking(ctx context.Context, bookingID uuid.UUID) (*BookingDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	result := toBookingDTO(bk)
	return &result, nil
}

// GetOwnerBookings retrieves paginated bookings for a specific owner.
func (s *BookingService) GetOwnerBookings(ctx context.Context, ownerID uuid.UUID, page, limit int) (*domain.PaginatedResult[BookingDTO], error) {
	bookings, total, err := s.repo.FindByOwnerID(ctx, ownerID, page, limit)
	if err != nil {
		return nil, err
	}

	dtos := make([]BookingDTO, len(bookings))
	for i, bk := range bookings {
		dtos[i] = toBookingDTO(bk)
	}

	result := domain.NewPaginatedResult(dtos, total, page, limit)
	return &result, nil
}

// GetRunnerBookings retrieves paginated bookings for a specific runner.
func (s *BookingService) GetRunnerBookings(ctx context.Context, runnerID uuid.UUID, page, limit int) (*domain.PaginatedResult[BookingDTO], error) {
	bookings, total, err := s.repo.FindByRunnerID(ctx, runnerID, page, limit)
	if err != nil {
		return nil, err
	}

	dtos := make([]BookingDTO, len(bookings))
	for i, bk := range bookings {
		dtos[i] = toBookingDTO(bk)
	}

	result := domain.NewPaginatedResult(dtos, total, page, limit)
	return &result, nil
}

// GetJobHistory retrieves completed and cancelled bookings for the caller's role.
func (s *BookingService) GetJobHistory(ctx context.Context, userID uuid.UUID, role string, page, limit int) (*domain.PaginatedResult[BookingDTO], error) {
	var (
		bookings []*bookingDomain.Booking
		total    int64
		err      error
	)
	switch role {
	case string(authRoleRunner):
		bookings, total, err = s.repo.FindHistoryByRunnerID(ctx, userID, page, limit)
	default:
		bookings, total, err = s.repo.FindHistoryByOwnerID(ctx, userID, page, limit)
	}
	if err != nil {
		return nil, err
	}
	return paginatedBookings(bookings, total, page, limit), nil
}

// GetScheduledBookings retrieves future scheduled bookings for the caller's role.
func (s *BookingService) GetScheduledBookings(ctx context.Context, userID uuid.UUID, role string, page, limit int) (*domain.PaginatedResult[BookingDTO], error) {
	now := time.Now().UTC()
	var (
		bookings []*bookingDomain.Booking
		total    int64
		err      error
	)
	switch role {
	case string(authRoleRunner):
		bookings, total, err = s.repo.FindScheduledByRunnerID(ctx, userID, now, page, limit)
	default:
		bookings, total, err = s.repo.FindScheduledByOwnerID(ctx, userID, now, page, limit)
	}
	if err != nil {
		return nil, err
	}
	return paginatedBookings(bookings, total, page, limit), nil
}

// RebookBooking clones an existing booking's data into a new booking.
func (s *BookingService) RebookBooking(ctx context.Context, ownerID, originalBookingID uuid.UUID) (*BookingDTO, error) {
	original, err := s.repo.FindByID(ctx, originalBookingID)
	if err != nil {
		return nil, err
	}

	// Only the original owner can rebook
	if original.OwnerID() != ownerID {
		return nil, domain.NewForbiddenError("booking does not belong to this user")
	}

	// Convert PetSpecification back to DTO
	petSpec := original.PetSpec()
	vaccDTOs := make([]dto.VaccinationDTO, len(petSpec.Vaccinations))
	for i, v := range petSpec.Vaccinations {
		vaccDTOs[i] = dto.VaccinationDTO{
			VaccineName: v.VaccineName,
			DateGiven:   v.DateGiven,
			ExpiresAt:   v.ExpiresAt,
			VetName:     v.VetName,
			Verified:    v.Verified,
		}
	}

	req := CreateBookingRequest{
		PetSpec: dto.PetSpecDTO{
			PetType:      petSpec.PetType,
			Breed:        petSpec.Breed,
			Name:         petSpec.Name,
			WeightKg:     petSpec.WeightKg,
			Age:          petSpec.Age,
			Vaccinations: vaccDTOs,
			SpecialNeeds: petSpec.SpecialNeeds,
			PhotoURL:     petSpec.PhotoURL,
		},
		PickupAddress:  original.PickupAddress(),
		DropoffAddress: original.DropoffAddress(),
		Notes:          original.Notes(),
	}

	return s.CreateBooking(ctx, ownerID, req)
}

// DeclineBooking allows an authenticated runner to decline a booking they accepted.
// It persists the reason and transitions the booking back to "requested" in a single transaction.
func (s *BookingService) DeclineBooking(ctx context.Context, bookingID, runnerID uuid.UUID, reason string) (*BookingDTO, error) {
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	// Authorization: the caller must be the runner assigned to this booking.
	if bk.RunnerID() == nil || *bk.RunnerID() != runnerID {
		return nil, domain.NewForbiddenError("not your booking")
	}

	// Guard: only accepted bookings may be declined; anything else → 409.
	if bk.Status() != bookingDomain.StatusAccepted {
		return nil, domain.NewConflictError(
			fmt.Sprintf("cannot decline booking in state '%s'", bk.Status()),
		)
	}

	// Apply domain transition (clears runnerID, sets status back to requested).
	if err := bk.Decline(reason); err != nil {
		return nil, err
	}
	bk.IncrementVersion()

	// Persist booking update + decline reason in a single transaction.
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update booking via the existing repo, which uses s.db internally.
		// We pass a tx-scoped repo so both writes share the same transaction.
		txBookingRepo := repository.NewGormBookingRepository(tx)
		if err := txBookingRepo.Update(ctx, bk); err != nil {
			return err
		}

		if err := s.declineRepo.RecordDecline(ctx, tx, bookingID, runnerID, reason); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// TODO(decline-event): re-emit booking.requested or add a new booking.declined event
	// so service-notification and any future dispatcher can re-offer this booking.
	// Tracked as a follow-up to Phase 3 of the app-runner design system plan.

	result := toBookingDTO(bk)
	return &result, nil
}

// --- Admin methods ---

// BookingStatsDTO holds booking statistics for the admin dashboard.
type BookingStatsDTO struct {
	TotalBookings int64            `json:"total_bookings"`
	ByStatus      map[string]int64 `json:"by_status"`
}

// ListAllBookings returns a paginated list of all bookings (admin).
func (s *BookingService) ListAllBookings(ctx context.Context, page, limit int) ([]BookingDTO, int64, error) {
	bookings, total, err := s.repo.ListAll(ctx, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list bookings: %w", err)
	}

	dtos := make([]BookingDTO, len(bookings))
	for i, bk := range bookings {
		dtos[i] = toBookingDTO(bk)
	}
	return dtos, total, nil
}

// GetBookingStats returns aggregate booking statistics (admin).
func (s *BookingService) GetBookingStats(ctx context.Context) (*BookingStatsDTO, error) {
	counts, err := s.repo.CountByStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking stats: %w", err)
	}

	var total int64
	for _, c := range counts {
		total += c
	}

	return &BookingStatsDTO{
		TotalBookings: total,
		ByStatus:      counts,
	}, nil
}

// --- Helpers ---

func toBookingDTO(bk *bookingDomain.Booking) BookingDTO {
	return BookingDTO{
		ID:                  bk.ID(),
		BookingNumber:       bk.BookingNumber(),
		OwnerID:             bk.OwnerID(),
		RunnerID:            bk.RunnerID(),
		Status:              string(bk.Status()),
		PetSpec:             bk.PetSpec(),
		CrateReq:            bk.CrateReq(),
		PickupAddress:       bk.PickupAddress(),
		DropoffAddress:      bk.DropoffAddress(),
		RouteSpec:           bk.RouteSpec(),
		EstimatedPriceCents: bk.EstimatedPriceCents(),
		FinalPriceCents:     bk.FinalPriceCents(),
		Currency:            bk.Currency(),
		ScheduledAt:         bk.ScheduledAt(),
		ArrivedAtPickup:     bk.ArrivedAtPickup(),
		PickedUpAt:          bk.PickedUpAt(),
		DeliveredAt:         bk.DeliveredAt(),
		CancelledAt:         bk.CancelledAt(),
		CancelNote:          bk.CancelNote(),
		Notes:               bk.Notes(),
		Version:             bk.Version(),
		CreatedAt:           bk.CreatedAt(),
		UpdatedAt:           bk.UpdatedAt(),
	}
}

func toProofDTO(proof *bookingDomain.ProofOfDelivery) ProofOfDeliveryDTO {
	return ProofOfDeliveryDTO{
		ID:            proof.ID(),
		BookingID:     proof.BookingID(),
		PhotoURL:      proof.PhotoURL(),
		SignatureURL:  proof.SignatureURL(),
		RecipientKind: proof.RecipientKind(),
		Notes:         proof.Notes(),
		CreatedAt:     proof.CreatedAt(),
	}
}

func paginatedBookings(bookings []*bookingDomain.Booking, total int64, page, limit int) *domain.PaginatedResult[BookingDTO] {
	dtos := make([]BookingDTO, len(bookings))
	for i, bk := range bookings {
		dtos[i] = toBookingDTO(bk)
	}
	result := domain.NewPaginatedResult(dtos, total, page, limit)
	return &result
}

func derefUUID(value *uuid.UUID) uuid.UUID {
	if value == nil {
		return uuid.Nil
	}
	return *value
}

func buildPetSpecification(petDTO dto.PetSpecDTO) bookingDomain.PetSpecification {
	vaccinations := make([]bookingDomain.VaccinationRecord, len(petDTO.Vaccinations))
	for i, v := range petDTO.Vaccinations {
		vaccinations[i] = bookingDomain.VaccinationRecord{
			VaccineName: v.VaccineName,
			DateGiven:   v.DateGiven,
			ExpiresAt:   v.ExpiresAt,
			VetName:     v.VetName,
			Verified:    v.Verified,
		}
	}

	return bookingDomain.PetSpecification{
		PetType:      petDTO.PetType,
		Breed:        petDTO.Breed,
		Name:         petDTO.Name,
		WeightKg:     petDTO.WeightKg,
		Age:          petDTO.Age,
		Vaccinations: vaccinations,
		SpecialNeeds: petDTO.SpecialNeeds,
		PhotoURL:     petDTO.PhotoURL,
	}
}

func (s *BookingService) publishBookingRequested(ctx context.Context, bk *bookingDomain.Booking) {
	evt := events.BookingRequestedEvent{
		BookingID:      bk.ID(),
		BookingNumber:  bk.BookingNumber(),
		OwnerID:        bk.OwnerID(),
		PetType:        bk.PetSpec().PetType,
		PetName:        bk.PetSpec().Name,
		PickupLat:      bk.PickupAddress().Latitude,
		PickupLng:      bk.PickupAddress().Longitude,
		DropoffLat:     bk.DropoffAddress().Latitude,
		DropoffLng:     bk.DropoffAddress().Longitude,
		EstimatedPrice: bk.EstimatedPriceCents(),
		Currency:       bk.Currency(),
		OccurredAt:     time.Now().UTC(),
	}
	s.publishEvent(ctx, events.TopicBookingEvents, events.BookingRequested, bk.ID().String(), evt)
}

func (s *BookingService) publishEvent(ctx context.Context, topic, eventType, key string, data interface{}) {
	if s.producer == nil {
		return
	}
	cloudEvent, err := kafka.NewCloudEvent("service-booking", eventType, data)
	if err != nil {
		s.logger.Error("failed to create cloud event",
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		return
	}

	if err := s.producer.PublishEvent(ctx, topic, cloudEvent); err != nil {
		s.logger.Error("failed to publish event",
			zap.String("topic", topic),
			zap.String("event_type", eventType),
			zap.Error(err),
		)
	}
}

// haversineDistance calculates the distance between two coordinates in kilometers.
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371.0

	dLat := degreesToRadians(lat2 - lat1)
	dLng := degreesToRadians(lng2 - lng1)

	lat1Rad := degreesToRadians(lat1)
	lat2Rad := degreesToRadians(lat2)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLng/2)*math.Sin(dLng/2)*math.Cos(lat1Rad)*math.Cos(lat2Rad)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}
