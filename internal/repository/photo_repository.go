package repository

import (
	"context"
	"time"

	photoDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/photo"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PhotoModel is the GORM model for the booking_photos table.
type PhotoModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	BookingID uuid.UUID `gorm:"type:uuid;not null;index"`
	RunnerID  uuid.UUID `gorm:"type:uuid;not null"`
	PhotoType string    `gorm:"type:varchar(20);not null"`
	PhotoURL  string    `gorm:"type:text;not null"`
	Caption   string    `gorm:"type:text"`
	TakenAt   time.Time `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
}

// TableName sets the table name.
func (PhotoModel) TableName() string { return "booking_photos" }

// GormPhotoRepository implements PhotoRepository using GORM.
type GormPhotoRepository struct {
	db *gorm.DB
}

// NewGormPhotoRepository creates a new GormPhotoRepository.
func NewGormPhotoRepository(db *gorm.DB) *GormPhotoRepository {
	return &GormPhotoRepository{db: db}
}

// Save persists a new booking photo.
func (r *GormPhotoRepository) Save(ctx context.Context, photo *photoDomain.BookingPhoto) error {
	model := toPhotoModel(photo)
	return r.db.WithContext(ctx).Create(&model).Error
}

// FindByBookingID returns all photos for a booking.
func (r *GormPhotoRepository) FindByBookingID(ctx context.Context, bookingID uuid.UUID) ([]*photoDomain.BookingPhoto, error) {
	var models []PhotoModel
	if err := r.db.WithContext(ctx).Where("booking_id = ?", bookingID).Order("taken_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	photos := make([]*photoDomain.BookingPhoto, len(models))
	for i, m := range models {
		photos[i] = toPhotoDomain(&m)
	}
	return photos, nil
}

// FindByID returns a single photo by ID.
func (r *GormPhotoRepository) FindByID(ctx context.Context, id uuid.UUID) (*photoDomain.BookingPhoto, error) {
	var model PhotoModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		return nil, err
	}
	return toPhotoDomain(&model), nil
}

func toPhotoModel(p *photoDomain.BookingPhoto) PhotoModel {
	return PhotoModel{
		ID:        p.ID(),
		BookingID: p.BookingID(),
		RunnerID:  p.RunnerID(),
		PhotoType: string(p.PhotoType()),
		PhotoURL:  p.PhotoURL(),
		Caption:   p.Caption(),
		TakenAt:   p.TakenAt(),
		CreatedAt: p.CreatedAt(),
	}
}

func toPhotoDomain(m *PhotoModel) *photoDomain.BookingPhoto {
	return photoDomain.Reconstruct(
		m.ID,
		m.BookingID,
		m.RunnerID,
		photoDomain.PhotoType(m.PhotoType),
		m.PhotoURL,
		m.Caption,
		m.TakenAt,
		m.CreatedAt,
	)
}
