package application

import (
	"context"
	"fmt"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/domain"
	petDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/pet"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CreatePetRequest is the request DTO for creating a pet profile.
type CreatePetRequest struct {
	Name              string  `json:"name" binding:"required"`
	PetType           string  `json:"pet_type" binding:"required"`
	Breed             string  `json:"breed"`
	WeightKg          float64 `json:"weight_kg"`
	AgeMonths         int     `json:"age_months"`
	Allergies         string  `json:"allergies"`
	SpecialNeeds      string  `json:"special_needs"`
	Notes             string  `json:"notes"`
	PhotoURL          string  `json:"photo_url"`
	VaccinationStatus string  `json:"vaccination_status"`
}

// UpdatePetRequest is the request DTO for updating a pet profile.
type UpdatePetRequest struct {
	Name              string  `json:"name"`
	PetType           string  `json:"pet_type"`
	Breed             string  `json:"breed"`
	WeightKg          float64 `json:"weight_kg"`
	AgeMonths         int     `json:"age_months"`
	Allergies         string  `json:"allergies"`
	SpecialNeeds      string  `json:"special_needs"`
	Notes             string  `json:"notes"`
	PhotoURL          string  `json:"photo_url"`
	VaccinationStatus string  `json:"vaccination_status"`
}

// PetDTO is the API response representation of a pet profile.
type PetDTO struct {
	ID                uuid.UUID `json:"id"`
	OwnerID           uuid.UUID `json:"owner_id"`
	Name              string    `json:"name"`
	PetType           string    `json:"pet_type"`
	Breed             string    `json:"breed"`
	WeightKg          float64   `json:"weight_kg"`
	AgeMonths         int       `json:"age_months"`
	Allergies         string    `json:"allergies,omitempty"`
	SpecialNeeds      string    `json:"special_needs,omitempty"`
	Notes             string    `json:"notes,omitempty"`
	PhotoURL          string    `json:"photo_url,omitempty"`
	VaccinationStatus string    `json:"vaccination_status,omitempty"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// PetService implements use cases for pet profile management.
type PetService struct {
	repo   petDomain.PetRepository
	logger *zap.Logger
}

// NewPetService creates a new PetService.
func NewPetService(repo petDomain.PetRepository, logger *zap.Logger) *PetService {
	return &PetService{repo: repo, logger: logger}
}

// CreatePet creates a new pet profile for the given owner.
func (s *PetService) CreatePet(ctx context.Context, ownerID uuid.UUID, req CreatePetRequest) (*PetDTO, error) {
	pet, err := petDomain.NewPet(
		ownerID,
		req.Name, req.PetType, req.Breed,
		req.WeightKg, req.AgeMonths,
		req.Allergies, req.SpecialNeeds, req.Notes,
		req.PhotoURL, req.VaccinationStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid pet data: %w", err)
	}

	if err := s.repo.Save(ctx, pet); err != nil {
		s.logger.Error("failed to create pet", zap.Error(err))
		return nil, fmt.Errorf("failed to create pet: %w", err)
	}

	s.logger.Info("pet profile created",
		zap.String("pet_id", pet.ID().String()),
		zap.String("owner_id", ownerID.String()),
	)
	result := toPetDTO(pet)
	return &result, nil
}

// GetMyPets returns all active pet profiles for the given owner.
func (s *PetService) GetMyPets(ctx context.Context, ownerID uuid.UUID) ([]PetDTO, error) {
	pets, err := s.repo.FindByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pets: %w", err)
	}
	dtos := make([]PetDTO, len(pets))
	for i, p := range pets {
		dtos[i] = toPetDTO(p)
	}
	return dtos, nil
}

// GetPet returns a single pet profile by ID, verifying ownership.
func (s *PetService) GetPet(ctx context.Context, ownerID, petID uuid.UUID) (*PetDTO, error) {
	pet, err := s.repo.FindByID(ctx, petID)
	if err != nil {
		return nil, err
	}
	if !pet.IsOwnedBy(ownerID) {
		return nil, domain.NewForbiddenError("you do not own this pet profile")
	}
	result := toPetDTO(pet)
	return &result, nil
}

// UpdatePet updates a pet profile, verifying ownership.
func (s *PetService) UpdatePet(ctx context.Context, ownerID, petID uuid.UUID, req UpdatePetRequest) (*PetDTO, error) {
	pet, err := s.repo.FindByID(ctx, petID)
	if err != nil {
		return nil, err
	}
	if !pet.IsOwnedBy(ownerID) {
		return nil, domain.NewForbiddenError("you do not own this pet profile")
	}

	pet.Update(
		req.Name, req.PetType, req.Breed,
		req.WeightKg, req.AgeMonths,
		req.Allergies, req.SpecialNeeds, req.Notes,
		req.PhotoURL, req.VaccinationStatus,
	)

	if err := s.repo.Update(ctx, pet); err != nil {
		s.logger.Error("failed to update pet", zap.Error(err))
		return nil, fmt.Errorf("failed to update pet: %w", err)
	}

	s.logger.Info("pet profile updated", zap.String("pet_id", petID.String()))
	result := toPetDTO(pet)
	return &result, nil
}

// DeletePet deletes a pet profile (or archives it), verifying ownership.
func (s *PetService) DeletePet(ctx context.Context, ownerID, petID uuid.UUID) error {
	pet, err := s.repo.FindByID(ctx, petID)
	if err != nil {
		return err
	}
	if !pet.IsOwnedBy(ownerID) {
		return domain.NewForbiddenError("you do not own this pet profile")
	}

	pet.Archive()
	if err := s.repo.Update(ctx, pet); err != nil {
		s.logger.Error("failed to archive pet", zap.Error(err))
		return fmt.Errorf("failed to archive pet: %w", err)
	}

	s.logger.Info("pet profile archived", zap.String("pet_id", petID.String()))
	return nil
}

func toPetDTO(p *petDomain.Pet) PetDTO {
	return PetDTO{
		ID:                p.ID(),
		OwnerID:           p.OwnerID(),
		Name:              p.Name(),
		PetType:           p.PetType(),
		Breed:             p.Breed(),
		WeightKg:          p.WeightKg(),
		AgeMonths:         p.AgeMonths(),
		Allergies:         p.Allergies(),
		SpecialNeeds:      p.SpecialNeeds(),
		Notes:             p.Notes(),
		PhotoURL:          p.PhotoURL(),
		VaccinationStatus: p.VaccinationStatus(),
		Status:            string(p.Status()),
		CreatedAt:         p.CreatedAt(),
		UpdatedAt:         p.UpdatedAt(),
	}
}
