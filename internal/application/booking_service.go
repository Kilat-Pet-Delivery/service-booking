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
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreateBookingRequest holds the data needed to create a new booking.
type CreateBookingRequest struct {
	PetSpec        dto.PetSpecDTO  `json:"pet_spec" binding:"required"`
	PickupAddress  dto.AddressDTO  `json:"pickup_address" binding:"required"`
	DropoffAddress dto.AddressDTO  `json:"dropoff_address" binding:"required"`
	ScheduledAt    *time.Time      `json:"scheduled_at"`
	Notes          string          `json:"notes"`
}

// BookingDTO is the response representation of a booking.
type BookingDTO struct {
	ID                  uuid.UUID              `json:"id"`
	BookingNumber       string                 `json:"booking_number"`
	OwnerID             uuid.UUID              `json:"owner_id"`
	RunnerID            *uuid.UUID             `json:"runner_id,omitempty"`
	Status              string                 `json:"status"`
	PetSpec             bookingDomain.PetSpecification  `json:"pet_spec"`
	CrateReq            bookingDomain.CrateRequirement  `json:"crate_requirement"`
	PickupAddress       dto.AddressDTO         `json:"pickup_address"`
	DropoffAddress      dto.AddressDTO         `json:"dropoff_address"`
	RouteSpec           *bookingDomain.RouteSpecification `json:"route_spec,omitempty"`
	EstimatedPriceCents int64                  `json:"estimated_price_cents"`
	FinalPriceCents     *int64                 `json:"final_price_cents,omitempty"`
	Currency            string                 `json:"currency"`
	ScheduledAt         *time.Time             `json:"scheduled_at,omitempty"`
	PickedUpAt          *time.Time             `json:"picked_up_at,omitempty"`
	DeliveredAt         *time.Time             `json:"delivered_at,omitempty"`
	CancelledAt         *time.Time             `json:"cancelled_at,omitempty"`
	CancelNote          string                 `json:"cancel_note,omitempty"`
	Notes               string                 `json:"notes,omitempty"`
	Version             int64                  `json:"version"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

// BookingService is the application service orchestrating booking use cases.
type BookingService struct {
	repo     bookingDomain.BookingRepository
	pricing  bookingDomain.PricingStrategy
	producer *kafka.Producer
	logger   *zap.Logger
}

// NewBookingService creates a new BookingService.
func NewBookingService(
	repo bookingDomain.BookingRepository,
	pricing bookingDomain.PricingStrategy,
	producer *kafka.Producer,
	logger *zap.Logger,
) *BookingService {
	return &BookingService{
		repo:     repo,
		pricing:  pricing,
		producer: producer,
		logger:   logger,
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
