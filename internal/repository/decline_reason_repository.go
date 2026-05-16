package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeclineReasonModel is the GORM model for the booking_decline_reasons table.
type DeclineReasonModel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	BookingID  uuid.UUID `gorm:"type:uuid;not null;index"`
	RunnerID   uuid.UUID `gorm:"type:uuid;not null;index"`
	Reason     string    `gorm:"type:text;not null"`
	DeclinedAt time.Time `gorm:"not null;default:now()"`
}

// TableName returns the table name for the GORM model.
func (DeclineReasonModel) TableName() string {
	return "booking_decline_reasons"
}

// GormDeclineReasonRepository persists runner decline reason records.
type GormDeclineReasonRepository struct {
	db *gorm.DB
}

// NewGormDeclineReasonRepository creates a new GormDeclineReasonRepository.
func NewGormDeclineReasonRepository(db *gorm.DB) *GormDeclineReasonRepository {
	return &GormDeclineReasonRepository{db: db}
}

// RecordDecline inserts a new decline reason row using the provided db handle
// (may be a transaction-scoped *gorm.DB or the main db).
func (r *GormDeclineReasonRepository) RecordDecline(ctx context.Context, db *gorm.DB, bookingID, runnerID uuid.UUID, reason string) error {
	model := &DeclineReasonModel{
		ID:         uuid.New(),
		BookingID:  bookingID,
		RunnerID:   runnerID,
		Reason:     reason,
		DeclinedAt: time.Now().UTC(),
	}
	if err := db.WithContext(ctx).Create(model).Error; err != nil {
		return fmt.Errorf("failed to record decline reason: %w", err)
	}
	return nil
}
