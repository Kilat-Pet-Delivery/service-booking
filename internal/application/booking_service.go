package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/domain"
	"github.com/Kilat-Pet-Delivery/lib-common/kafka"
	"github.com/Kilat-Pet-Delivery/lib-proto/dto"
	"github.com/Kilat-Pet-Delivery/lib-proto/events"
	bookingDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/booking"
	"github.com/Kilat-Pet-Delivery/service-booking/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CreateBookingRequest holds the data needed to create a new booking.
type CreateBookingRequest struct {
	PetSpec        dto.PetSpecDTO       `json:"pet_spec" binding:"required"`
	PickupAddress  dto.AddressDTO       `json:"pickup_address" binding:"required"`
	DropoffAddress dto.AddressDTO       `json:"dropoff_address" binding:"required"`
	ShopID         *uuid.UUID           `json:"shop_id,omitempty"`
	Items          []BookingItemRequest `json:"items,omitempty"`
	ScheduledAt    *time.Time           `json:"scheduled_at"`
	Notes          string               `json:"notes"`
}

// BookingItemRequest is one explicit shop line item.
type BookingItemRequest struct {
	ProductID     uuid.UUID `json:"product_id" binding:"required"`
	Qty           int64     `json:"qty" binding:"required"`
	PriceMyrCents int64     `json:"price_myr_cents" binding:"required"`
	SKU           string    `json:"sku,omitempty"`
	Name          string    `json:"name,omitempty"`
}

// BookingDTO is the response representation of a booking.
type BookingDTO struct {
	ID                  uuid.UUID                         `json:"id"`
	BookingNumber       string                            `json:"booking_number"`
	OwnerID             uuid.UUID                         `json:"owner_id"`
	RunnerID            *uuid.UUID                        `json:"runner_id,omitempty"`
	ShopID              *uuid.UUID                        `json:"shop_id,omitempty"`
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
	PickedUpAt          *time.Time                        `json:"picked_up_at,omitempty"`
	DeliveredAt         *time.Time                        `json:"delivered_at,omitempty"`
	CancelledAt         *time.Time                        `json:"cancelled_at,omitempty"`
	CancelNote          string                            `json:"cancel_note,omitempty"`
	Notes               string                            `json:"notes,omitempty"`
	QRPickupToken       *string                           `json:"qr_pickup_token,omitempty"`
	Version             int64                             `json:"version"`
	CreatedAt           time.Time                         `json:"created_at"`
	UpdatedAt           time.Time                         `json:"updated_at"`
}

// BookingService is the application service orchestrating booking use cases.
type BookingService struct {
	repo        bookingDomain.BookingRepository
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
	return &BookingService{
		repo:        repo,
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
		req.ShopID,
	)
	if err != nil {
		return nil, err
	}

	// Persist the booking
	if err := s.repo.Save(ctx, bk); err != nil {
		return nil, fmt.Errorf("failed to save booking: %w", err)
	}
	if len(req.Items) > 0 {
		items := make([]bookingDomain.BookingItem, len(req.Items))
		for i, item := range req.Items {
			items[i] = bookingDomain.BookingItem{
				BookingID:     bk.ID(),
				ProductID:     item.ProductID,
				Qty:           item.Qty,
				PriceMyrCents: item.PriceMyrCents,
				SKU:           item.SKU,
				Name:          item.Name,
			}
		}
		if err := s.repo.SaveItems(ctx, bk.ID(), items); err != nil {
			return nil, err
		}
	}

	// Publish BookingRequestedEvent
	s.publishBookingRequested(ctx, bk)

	result := toBookingDTO(bk)
	return &result, nil
}

// AcceptByShop accepts an incoming shop order.
func (s *BookingService) AcceptByShop(ctx context.Context, bookingID, shopID, actorUserID uuid.UUID, key string) (*BookingDTO, error) {
	hash := idempotencyHash("shop-accept", bookingID.String(), shopID.String())
	replay, err := s.checkIdempotency(ctx, key, hash)
	if err != nil {
		return nil, err
	}
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if !replay {
		if err := bk.AcceptByShop(shopID); err != nil {
			return nil, err
		}
		bk.IncrementVersion()
		if err := s.repo.Update(ctx, bk); err != nil {
			return nil, err
		}
		s.publishEvent(ctx, events.TopicBookingEvents, events.BookingAcceptedByShop, bk.ID().String(), events.BookingAcceptedByShopEvent{
			BookingID:     bk.ID(),
			BookingNumber: bk.BookingNumber(),
			ShopID:        shopID,
			AcceptedBy:    actorUserID,
			OccurredAt:    time.Now().UTC(),
		})
		if err := s.markIdempotent(ctx, key, hash); err != nil {
			return nil, err
		}
	}
	result := toBookingDTO(bk)
	return &result, nil
}

// MarkPreparing moves a shop order into preparing.
func (s *BookingService) MarkPreparing(ctx context.Context, bookingID, shopID, actorUserID uuid.UUID, key string) (*BookingDTO, error) {
	hash := idempotencyHash("shop-preparing", bookingID.String(), shopID.String())
	replay, err := s.checkIdempotency(ctx, key, hash)
	if err != nil {
		return nil, err
	}
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if !replay {
		if err := bk.MarkPreparing(shopID); err != nil {
			return nil, err
		}
		bk.IncrementVersion()
		if err := s.repo.Update(ctx, bk); err != nil {
			return nil, err
		}
		s.publishEvent(ctx, events.TopicBookingEvents, events.BookingPreparing, bk.ID().String(), events.BookingPreparingEvent{
			BookingID:     bk.ID(),
			BookingNumber: bk.BookingNumber(),
			ShopID:        shopID,
			StartedBy:     actorUserID,
			OccurredAt:    time.Now().UTC(),
		})
		if err := s.markIdempotent(ctx, key, hash); err != nil {
			return nil, err
		}
	}
	result := toBookingDTO(bk)
	return &result, nil
}

// MarkReadyForPickup moves a shop order to ready_for_pickup and generates a QR token.
func (s *BookingService) MarkReadyForPickup(ctx context.Context, bookingID, shopID uuid.UUID, key string) (*BookingDTO, error) {
	hash := idempotencyHash("shop-ready", bookingID.String(), shopID.String())
	replay, err := s.checkIdempotency(ctx, key, hash)
	if err != nil {
		return nil, err
	}
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if !replay {
		token, err := bookingDomain.GenerateQRToken()
		if err != nil {
			return nil, err
		}
		if err := bk.MarkReadyForPickup(shopID, token); err != nil {
			return nil, err
		}
		bk.IncrementVersion()
		if err := s.repo.Update(ctx, bk); err != nil {
			return nil, err
		}
		s.publishEvent(ctx, events.TopicBookingEvents, events.BookingReadyForPickup, bk.ID().String(), events.BookingReadyForPickupEvent{
			BookingID:     bk.ID(),
			BookingNumber: bk.BookingNumber(),
			ShopID:        shopID,
			QRPickupToken: token,
			OccurredAt:    time.Now().UTC(),
		})
		if err := s.markIdempotent(ctx, key, hash); err != nil {
			return nil, err
		}
	}
	result := toBookingDTO(bk)
	return &result, nil
}

// VerifyPickup validates the QR token and starts the delivery for an assigned runner.
func (s *BookingService) VerifyPickup(ctx context.Context, bookingID, runnerID uuid.UUID, token, key string) (*BookingDTO, error) {
	hash := idempotencyHash("verify-pickup", bookingID.String(), runnerID.String(), token)
	replay, err := s.checkIdempotency(ctx, key, hash)
	if err != nil {
		return nil, err
	}
	bk, err := s.repo.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if !replay {
		if err := bk.VerifyPickup(runnerID, token); err != nil {
			return nil, err
		}
		bk.IncrementVersion()
		if err := s.repo.Update(ctx, bk); err != nil {
			return nil, err
		}
		if err := s.markIdempotent(ctx, key, hash); err != nil {
			return nil, err
		}
	}
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
	s.publishBookingDelivered(ctx, bk)

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
		ShopID:              bk.ShopID(),
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
		QRPickupToken:       bk.QRPickupToken(),
		Version:             bk.Version(),
		CreatedAt:           bk.CreatedAt(),
		UpdatedAt:           bk.UpdatedAt(),
	}
}

const BookingDelivered = "booking.delivered"

type bookingDeliveredEvent struct {
	BookingID       uuid.UUID                  `json:"booking_id"`
	BookingNumber   string                     `json:"booking_number"`
	ShopID          uuid.UUID                  `json:"shop_id"`
	DeliveredAt     time.Time                  `json:"delivered_at"`
	Currency        string                     `json:"currency"`
	GrossSalesCents int64                      `json:"gross_sales_cents"`
	NetSalesCents   int64                      `json:"net_sales_cents"`
	Items           []bookingDeliveredLineItem `json:"items"`
}

type bookingDeliveredLineItem struct {
	ProductID     uuid.UUID `json:"product_id"`
	SKU           string    `json:"sku"`
	Name          string    `json:"name"`
	Qty           int64     `json:"qty"`
	PriceMyrCents int64     `json:"price_myr_cents"`
}

func (s *BookingService) publishBookingDelivered(ctx context.Context, bk *bookingDomain.Booking) {
	if bk.ShopID() == nil || bk.DeliveredAt() == nil {
		return
	}
	items, err := s.repo.FindItemsByBookingID(ctx, bk.ID())
	if err != nil {
		s.logger.Error("failed to load booking items for delivered event", zap.Error(err))
		return
	}
	payloadItems := make([]bookingDeliveredLineItem, len(items))
	var gross int64
	for i, item := range items {
		payloadItems[i] = bookingDeliveredLineItem{
			ProductID:     item.ProductID,
			SKU:           item.SKU,
			Name:          item.Name,
			Qty:           item.Qty,
			PriceMyrCents: item.PriceMyrCents,
		}
		gross += item.Qty * item.PriceMyrCents
	}
	if gross == 0 {
		gross = bk.EstimatedPriceCents()
	}
	s.publishEvent(ctx, events.TopicBookingEvents, BookingDelivered, bk.ID().String(), bookingDeliveredEvent{
		BookingID:       bk.ID(),
		BookingNumber:   bk.BookingNumber(),
		ShopID:          *bk.ShopID(),
		DeliveredAt:     *bk.DeliveredAt(),
		Currency:        bk.Currency(),
		GrossSalesCents: gross,
		NetSalesCents:   gross,
		Items:           payloadItems,
	})
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

func (s *BookingService) checkIdempotency(ctx context.Context, key, requestHash string) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return false, domain.NewValidationError("Idempotency-Key header is required")
	}
	var row repository.IdempotencyKeyModel
	err := s.db.WithContext(ctx).Where("key = ?", key).Take(&row).Error
	if err == nil {
		if row.RequestHash != requestHash {
			return false, domain.NewConflictError("idempotency key reused with different request")
		}
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func (s *BookingService) markIdempotent(ctx context.Context, key, requestHash string) error {
	return s.db.WithContext(ctx).Create(&repository.IdempotencyKeyModel{
		Key:         key,
		RequestHash: requestHash,
		CreatedAt:   time.Now().UTC(),
	}).Error
}

func idempotencyHash(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
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
