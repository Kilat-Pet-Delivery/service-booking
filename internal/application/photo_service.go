package application

import (
	"context"
	"time"

	photoDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/photo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UploadPhotoRequest holds the data to upload a proof photo.
type UploadPhotoRequest struct {
	PhotoType string `json:"photo_type" binding:"required"`
	PhotoURL  string `json:"photo_url" binding:"required"`
	Caption   string `json:"caption"`
}

// PhotoDTO is the API response representation of a booking photo.
type PhotoDTO struct {
	ID        uuid.UUID `json:"id"`
	BookingID uuid.UUID `json:"booking_id"`
	RunnerID  uuid.UUID `json:"runner_id"`
	PhotoType string    `json:"photo_type"`
	PhotoURL  string    `json:"photo_url"`
	Caption   string    `json:"caption"`
	TakenAt   time.Time `json:"taken_at"`
	CreatedAt time.Time `json:"created_at"`
}

// PhotoService handles booking photo use cases.
type PhotoService struct {
	repo   photoDomain.PhotoRepository
	logger *zap.Logger
}

// NewPhotoService creates a new PhotoService.
func NewPhotoService(repo photoDomain.PhotoRepository, logger *zap.Logger) *PhotoService {
	return &PhotoService{repo: repo, logger: logger}
}

// UploadPhoto creates a new proof photo for a booking.
func (s *PhotoService) UploadPhoto(ctx context.Context, bookingID, runnerID uuid.UUID, req UploadPhotoRequest) (*PhotoDTO, error) {
	photo, err := photoDomain.NewBookingPhoto(
		bookingID,
		runnerID,
		photoDomain.PhotoType(req.PhotoType),
		req.PhotoURL,
		req.Caption,
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, photo); err != nil {
		return nil, err
	}

	s.logger.Info("photo uploaded",
		zap.String("booking_id", bookingID.String()),
		zap.String("photo_type", req.PhotoType),
	)

	return toPhotoDTO(photo), nil
}

// GetBookingPhotos returns all photos for a booking.
func (s *PhotoService) GetBookingPhotos(ctx context.Context, bookingID uuid.UUID) ([]*PhotoDTO, error) {
	photos, err := s.repo.FindByBookingID(ctx, bookingID)
	if err != nil {
		return nil, err
	}

	dtos := make([]*PhotoDTO, len(photos))
	for i, p := range photos {
		dtos[i] = toPhotoDTO(p)
	}
	return dtos, nil
}

func toPhotoDTO(p *photoDomain.BookingPhoto) *PhotoDTO {
	return &PhotoDTO{
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
