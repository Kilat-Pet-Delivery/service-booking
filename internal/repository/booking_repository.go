package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/domain"
	"github.com/Kilat-Pet-Delivery/lib-proto/dto"
	bookingDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/booking"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BookingModel is the GORM model for the bookings table.
type BookingModel struct {
	ID                  uuid.UUID       `gorm:"type:uuid;primaryKey"`
	BookingNumber       string          `gorm:"uniqueIndex;not null;size:20"`
	OwnerID             uuid.UUID       `gorm:"type:uuid;index;not null"`
	RunnerID            *uuid.UUID      `gorm:"type:uuid;index"`
	Status              string          `gorm:"not null;size:30;index"`
	PetSpec             json.RawMessage `gorm:"type:jsonb;not null"`
	CrateRequirement    json.RawMessage `gorm:"type:jsonb;not null"`
	PickupAddress       json.RawMessage `gorm:"type:jsonb;not null"`
	DropoffAddress      json.RawMessage `gorm:"type:jsonb;not null"`
	RouteSpec           json.RawMessage `gorm:"type:jsonb"`
	EstimatedPriceCents int64           `gorm:"not null"`
	FinalPriceCents     *int64          `gorm:""`
	Currency            string          `gorm:"not null;size:3;default:'MYR'"`
	ScheduledAt         *time.Time      `gorm:""`
	PickedUpAt          *time.Time      `gorm:""`
	DeliveredAt         *time.Time      `gorm:""`
	CancelledAt         *time.Time      `gorm:""`
	CancelNote          string          `gorm:"size:500"`
	Notes               string          `gorm:"size:1000"`
	Version             int64           `gorm:"not null;default:1"`
	CreatedAt           time.Time       `gorm:"not null"`
	UpdatedAt           time.Time       `gorm:"not null"`
}

// TableName returns the table name for the GORM model.
func (BookingModel) TableName() string {
	return "bookings"
}

// GormBookingRepository is the GORM-based implementation of BookingRepository.
type GormBookingRepository struct {
	db *gorm.DB
}

// NewGormBookingRepository creates a new GormBookingRepository.
func NewGormBookingRepository(db *gorm.DB) *GormBookingRepository {
	return &GormBookingRepository{db: db}
}

// FindByID retrieves a booking by its unique identifier.
func (r *GormBookingRepository) FindByID(ctx context.Context, id uuid.UUID) (*bookingDomain.Booking, error) {
	var model BookingModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("Booking", id.String())
		}
		return nil, fmt.Errorf("failed to find booking by ID: %w", err)
	}
	return toDomainBooking(&model)
}

// FindByNumber retrieves a booking by its booking number.
func (r *GormBookingRepository) FindByNumber(ctx context.Context, number string) (*bookingDomain.Booking, error) {
	var model BookingModel
	if err := r.db.WithContext(ctx).Where("booking_number = ?", number).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("Booking", number)
		}
		return nil, fmt.Errorf("failed to find booking by number: %w", err)
	}
	return toDomainBooking(&model)
}

// FindByOwnerID retrieves bookings for a specific owner with pagination.
func (r *GormBookingRepository) FindByOwnerID(ctx context.Context, ownerID uuid.UUID, page, limit int) ([]*bookingDomain.Booking, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&BookingModel{}).Where("owner_id = ?", ownerID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count owner bookings: %w", err)
	}

	var models []BookingModel
	offset := (page - 1) * limit
	if err := r.db.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to find owner bookings: %w", err)
	}

	bookings := make([]*bookingDomain.Booking, len(models))
	for i, m := range models {
		bk, err := toDomainBooking(&m)
		if err != nil {
			return nil, 0, err
		}
		bookings[i] = bk
	}

	return bookings, total, nil
}

// FindByRunnerID retrieves bookings for a specific runner with pagination.
func (r *GormBookingRepository) FindByRunnerID(ctx context.Context, runnerID uuid.UUID, page, limit int) ([]*bookingDomain.Booking, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&BookingModel{}).Where("runner_id = ?", runnerID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count runner bookings: %w", err)
	}

	var models []BookingModel
	offset := (page - 1) * limit
	if err := r.db.WithContext(ctx).
		Where("runner_id = ?", runnerID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to find runner bookings: %w", err)
	}

	bookings := make([]*bookingDomain.Booking, len(models))
	for i, m := range models {
		bk, err := toDomainBooking(&m)
		if err != nil {
			return nil, 0, err
		}
		bookings[i] = bk
	}

	return bookings, total, nil
}

// Save persists a new booking.
func (r *GormBookingRepository) Save(ctx context.Context, bk *bookingDomain.Booking) error {
	model, err := toBookingModel(bk)
	if err != nil {
		return fmt.Errorf("failed to convert booking to model: %w", err)
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return fmt.Errorf("failed to save booking: %w", err)
	}
	return nil
}

// Update persists changes to an existing booking with optimistic locking.
func (r *GormBookingRepository) Update(ctx context.Context, bk *bookingDomain.Booking) error {
	model, err := toBookingModel(bk)
	if err != nil {
		return fmt.Errorf("failed to convert booking to model: %w", err)
	}

	// Optimistic locking: only update if the version matches (current version - 1 since IncrementVersion was called)
	expectedVersion := bk.Version() - 1
	result := r.db.WithContext(ctx).
		Model(&BookingModel{}).
		Where("id = ? AND version = ?", model.ID, expectedVersion).
		Updates(map[string]interface{}{
			"runner_id":             model.RunnerID,
			"status":               model.Status,
			"pet_spec":             model.PetSpec,
			"crate_requirement":    model.CrateRequirement,
			"pickup_address":       model.PickupAddress,
			"dropoff_address":      model.DropoffAddress,
			"route_spec":           model.RouteSpec,
			"estimated_price_cents": model.EstimatedPriceCents,
			"final_price_cents":    model.FinalPriceCents,
			"currency":             model.Currency,
			"scheduled_at":         model.ScheduledAt,
			"picked_up_at":         model.PickedUpAt,
			"delivered_at":         model.DeliveredAt,
			"cancelled_at":         model.CancelledAt,
			"cancel_note":          model.CancelNote,
			"notes":                model.Notes,
			"version":              model.Version,
			"updated_at":           model.UpdatedAt,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update booking: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return domain.NewConflictError("booking was modified by another transaction")
	}

	return nil
}

// ListAll retrieves all bookings with pagination (admin).
func (r *GormBookingRepository) ListAll(ctx context.Context, page, limit int) ([]*bookingDomain.Booking, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&BookingModel{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count bookings: %w", err)
	}

	var models []BookingModel
	offset := (page - 1) * limit
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list bookings: %w", err)
	}

	bookings := make([]*bookingDomain.Booking, len(models))
	for i, m := range models {
		bk, err := toDomainBooking(&m)
		if err != nil {
			return nil, 0, err
		}
		bookings[i] = bk
	}

	return bookings, total, nil
}

// CountByStatus returns booking counts grouped by status (admin).
func (r *GormBookingRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	type statusCount struct {
		Status string
		Count  int64
	}
	var results []statusCount
	if err := r.db.WithContext(ctx).Model(&BookingModel{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to count by status: %w", err)
	}

	counts := make(map[string]int64)
	for _, sc := range results {
		counts[sc.Status] = sc.Count
	}
	return counts, nil
}

// --- Conversion Helpers ---

func toBookingModel(bk *bookingDomain.Booking) (*BookingModel, error) {
	petSpecJSON, err := json.Marshal(bk.PetSpec())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pet spec: %w", err)
	}

	crateReqJSON, err := json.Marshal(bk.CrateReq())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal crate requirement: %w", err)
	}

	pickupJSON, err := json.Marshal(bk.PickupAddress())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pickup address: %w", err)
	}

	dropoffJSON, err := json.Marshal(bk.DropoffAddress())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dropoff address: %w", err)
	}

	var routeSpecJSON json.RawMessage
	if bk.RouteSpec() != nil {
		data, err := json.Marshal(bk.RouteSpec())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal route spec: %w", err)
		}
		routeSpecJSON = data
	}

	return &BookingModel{
		ID:                  bk.ID(),
		BookingNumber:       bk.BookingNumber(),
		OwnerID:             bk.OwnerID(),
		RunnerID:            bk.RunnerID(),
		Status:              string(bk.Status()),
		PetSpec:             petSpecJSON,
		CrateRequirement:    crateReqJSON,
		PickupAddress:       pickupJSON,
		DropoffAddress:      dropoffJSON,
		RouteSpec:           routeSpecJSON,
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
	}, nil
}

func toDomainBooking(m *BookingModel) (*bookingDomain.Booking, error) {
	var petSpec bookingDomain.PetSpecification
	if err := json.Unmarshal(m.PetSpec, &petSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pet spec: %w", err)
	}

	var crateReq bookingDomain.CrateRequirement
	if err := json.Unmarshal(m.CrateRequirement, &crateReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal crate requirement: %w", err)
	}

	var pickupAddress dto.AddressDTO
	if err := json.Unmarshal(m.PickupAddress, &pickupAddress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pickup address: %w", err)
	}

	var dropoffAddress dto.AddressDTO
	if err := json.Unmarshal(m.DropoffAddress, &dropoffAddress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dropoff address: %w", err)
	}

	var routeSpec *bookingDomain.RouteSpecification
	if m.RouteSpec != nil && len(m.RouteSpec) > 0 {
		var rs bookingDomain.RouteSpecification
		if err := json.Unmarshal(m.RouteSpec, &rs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal route spec: %w", err)
		}
		routeSpec = &rs
	}

	status, err := bookingDomain.ParseBookingStatus(m.Status)
	if err != nil {
		return nil, err
	}

	return bookingDomain.ReconstructBooking(
		m.ID,
		m.BookingNumber,
		m.OwnerID,
		m.RunnerID,
		status,
		petSpec,
		crateReq,
		pickupAddress,
		dropoffAddress,
		routeSpec,
		m.EstimatedPriceCents,
		m.FinalPriceCents,
		m.Currency,
		m.ScheduledAt,
		m.PickedUpAt,
		m.DeliveredAt,
		m.CancelledAt,
		m.CancelNote,
		m.Notes,
		m.Version,
		m.CreatedAt,
		m.UpdatedAt,
	), nil
}
