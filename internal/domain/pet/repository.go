package pet

import (
	"context"

	"github.com/google/uuid"
)

// PetRepository defines persistence operations for pet profiles.
type PetRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Pet, error)
	FindByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]*Pet, error)
	Save(ctx context.Context, pet *Pet) error
	Update(ctx context.Context, pet *Pet) error
	Delete(ctx context.Context, id uuid.UUID) error
}
