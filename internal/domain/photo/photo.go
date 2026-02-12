package photo

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PhotoType represents the type of proof photo.
type PhotoType string

const (
	PhotoTypePickup   PhotoType = "pickup"
	PhotoTypeDelivery PhotoType = "delivery"
)

// IsValid returns true if the photo type is recognized.
func (p PhotoType) IsValid() bool {
	return p == PhotoTypePickup || p == PhotoTypeDelivery
}

// BookingPhoto is the aggregate root for booking proof photos.
type BookingPhoto struct {
	id        uuid.UUID
	bookingID uuid.UUID
	runnerID  uuid.UUID
	photoType PhotoType
	photoURL  string
	caption   string
	takenAt   time.Time
	createdAt time.Time
}

// NewBookingPhoto creates a new booking photo.
func NewBookingPhoto(bookingID, runnerID uuid.UUID, photoType PhotoType, photoURL, caption string) (*BookingPhoto, error) {
	if !photoType.IsValid() {
		return nil, fmt.Errorf("invalid photo type: %s", photoType)
	}
	if photoURL == "" {
		return nil, fmt.Errorf("photo URL is required")
	}

	now := time.Now().UTC()
	return &BookingPhoto{
		id:        uuid.New(),
		bookingID: bookingID,
		runnerID:  runnerID,
		photoType: photoType,
		photoURL:  photoURL,
		caption:   caption,
		takenAt:   now,
		createdAt: now,
	}, nil
}

// Reconstruct rebuilds a BookingPhoto from persistence.
func Reconstruct(id, bookingID, runnerID uuid.UUID, photoType PhotoType, photoURL, caption string, takenAt, createdAt time.Time) *BookingPhoto {
	return &BookingPhoto{
		id:        id,
		bookingID: bookingID,
		runnerID:  runnerID,
		photoType: photoType,
		photoURL:  photoURL,
		caption:   caption,
		takenAt:   takenAt,
		createdAt: createdAt,
	}
}

// Getters.
func (p *BookingPhoto) ID() uuid.UUID        { return p.id }
func (p *BookingPhoto) BookingID() uuid.UUID  { return p.bookingID }
func (p *BookingPhoto) RunnerID() uuid.UUID   { return p.runnerID }
func (p *BookingPhoto) PhotoType() PhotoType  { return p.photoType }
func (p *BookingPhoto) PhotoURL() string      { return p.photoURL }
func (p *BookingPhoto) Caption() string       { return p.caption }
func (p *BookingPhoto) TakenAt() time.Time    { return p.takenAt }
func (p *BookingPhoto) CreatedAt() time.Time  { return p.createdAt }
