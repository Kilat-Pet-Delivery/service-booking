package photo

import (
	"context"

	"github.com/google/uuid"
)

// PhotoRepository defines persistence operations for booking photos.
type PhotoRepository interface {
	Save(ctx context.Context, photo *BookingPhoto) error
	FindByBookingID(ctx context.Context, bookingID uuid.UUID) ([]*BookingPhoto, error)
	FindByID(ctx context.Context, id uuid.UUID) (*BookingPhoto, error)
}
