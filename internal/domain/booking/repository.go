package booking

import (
	"context"

	"github.com/google/uuid"
)

// BookingRepository defines the persistence contract for booking aggregates.
type BookingRepository interface {
	// FindByID retrieves a booking by its unique identifier.
	FindByID(ctx context.Context, id uuid.UUID) (*Booking, error)

	// FindByNumber retrieves a booking by its human-readable booking number.
	FindByNumber(ctx context.Context, number string) (*Booking, error)

	// FindByOwnerID retrieves bookings belonging to a specific owner with pagination.
	FindByOwnerID(ctx context.Context, ownerID uuid.UUID, page, limit int) ([]*Booking, int64, error)

	// FindByRunnerID retrieves bookings assigned to a specific runner with pagination.
	FindByRunnerID(ctx context.Context, runnerID uuid.UUID, page, limit int) ([]*Booking, int64, error)

	// ListAll retrieves all bookings with pagination (admin).
	ListAll(ctx context.Context, page, limit int) ([]*Booking, int64, error)

	// CountByStatus returns booking counts grouped by status (admin).
	CountByStatus(ctx context.Context) (map[string]int64, error)

	// Save persists a new booking.
	Save(ctx context.Context, booking *Booking) error

	// Update persists changes to an existing booking with optimistic locking.
	Update(ctx context.Context, booking *Booking) error
}
