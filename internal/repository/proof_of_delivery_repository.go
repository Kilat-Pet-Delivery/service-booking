package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/domain"
	bookingDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/booking"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProofOfDeliveryModel is the GORM model for proofs_of_delivery.
type ProofOfDeliveryModel struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	BookingID     uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	PhotoURL      string    `gorm:"type:text;not null"`
	SignatureURL  string    `gorm:"type:text;not null"`
	RecipientKind string    `gorm:"type:varchar(30);not null"`
	Notes         string    `gorm:"type:text"`
	CreatedAt     time.Time `gorm:"not null"`
}

func (ProofOfDeliveryModel) TableName() string {
	return "proofs_of_delivery"
}

// GormProofOfDeliveryRepository persists delivery proofs.
type GormProofOfDeliveryRepository struct {
	db *gorm.DB
}

// NewGormProofOfDeliveryRepository creates a proof repository.
func NewGormProofOfDeliveryRepository(db *gorm.DB) *GormProofOfDeliveryRepository {
	return &GormProofOfDeliveryRepository{db: db}
}

func (r *GormProofOfDeliveryRepository) Save(ctx context.Context, proof *bookingDomain.ProofOfDelivery) error {
	return r.db.WithContext(ctx).Create(proofToModel(proof)).Error
}

func (r *GormProofOfDeliveryRepository) FindByBookingID(ctx context.Context, bookingID uuid.UUID) (*bookingDomain.ProofOfDelivery, error) {
	var model ProofOfDeliveryModel
	if err := r.db.WithContext(ctx).Where("booking_id = ?", bookingID).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return proofToDomain(&model), nil
}

func proofToModel(proof *bookingDomain.ProofOfDelivery) *ProofOfDeliveryModel {
	return &ProofOfDeliveryModel{
		ID:            proof.ID(),
		BookingID:     proof.BookingID(),
		PhotoURL:      proof.PhotoURL(),
		SignatureURL:  proof.SignatureURL(),
		RecipientKind: proof.RecipientKind(),
		Notes:         proof.Notes(),
		CreatedAt:     proof.CreatedAt(),
	}
}

func proofToDomain(model *ProofOfDeliveryModel) *bookingDomain.ProofOfDelivery {
	return bookingDomain.ReconstructProofOfDelivery(
		model.ID,
		model.BookingID,
		model.PhotoURL,
		model.SignatureURL,
		model.RecipientKind,
		model.Notes,
		model.CreatedAt,
	)
}
