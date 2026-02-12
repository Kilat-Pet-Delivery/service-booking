package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Kilat-Pet-Delivery/lib-common/domain"
	petDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/pet"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PetModel is the GORM model for the pets table.
type PetModel struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	OwnerID           uuid.UUID `gorm:"type:uuid;not null;index"`
	Name              string    `gorm:"type:varchar(100);not null"`
	PetType           string    `gorm:"type:varchar(20);not null"`
	Breed             string    `gorm:"type:varchar(100)"`
	WeightKg          float64   `gorm:"type:decimal(5,2)"`
	AgeMonths         int       `gorm:"type:int"`
	Allergies         string    `gorm:"type:text"`
	SpecialNeeds      string    `gorm:"type:text"`
	Notes             string    `gorm:"type:text"`
	PhotoURL          string    `gorm:"type:text"`
	VaccinationStatus string    `gorm:"type:varchar(50)"`
	Status            string    `gorm:"type:varchar(20);not null;default:'active'"`
	Version           int64     `gorm:"not null;default:1"`
	CreatedAt         time.Time `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt         time.Time `gorm:"type:timestamptz;not null;default:now()"`
}

func (PetModel) TableName() string { return "pets" }

// GormPetRepository implements PetRepository using GORM.
type GormPetRepository struct {
	db *gorm.DB
}

func NewGormPetRepository(db *gorm.DB) *GormPetRepository {
	return &GormPetRepository{db: db}
}

func (r *GormPetRepository) FindByID(ctx context.Context, id uuid.UUID) (*petDomain.Pet, error) {
	var model PetModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("Pet", id.String())
		}
		return nil, err
	}
	return toPetDomain(&model), nil
}

func (r *GormPetRepository) FindByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]*petDomain.Pet, error) {
	var models []PetModel
	if err := r.db.WithContext(ctx).
		Where("owner_id = ? AND status = ?", ownerID, "active").
		Order("created_at DESC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	pets := make([]*petDomain.Pet, len(models))
	for i, m := range models {
		pets[i] = toPetDomain(&m)
	}
	return pets, nil
}

func (r *GormPetRepository) Save(ctx context.Context, pet *petDomain.Pet) error {
	model := toPetModel(pet)
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *GormPetRepository) Update(ctx context.Context, pet *petDomain.Pet) error {
	model := toPetModel(pet)
	previousVersion := pet.Version() - 1

	result := r.db.WithContext(ctx).
		Model(&PetModel{}).
		Where("id = ? AND version = ?", model.ID, previousVersion).
		Updates(model)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.NewConflictError("pet was modified by another transaction")
	}
	return nil
}

func (r *GormPetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&PetModel{}).Error
}

// --- Conversions ---

func toPetModel(p *petDomain.Pet) *PetModel {
	return &PetModel{
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
		Version:           p.Version(),
		CreatedAt:         p.CreatedAt(),
		UpdatedAt:         p.UpdatedAt(),
	}
}

func toPetDomain(m *PetModel) *petDomain.Pet {
	return petDomain.Reconstruct(
		m.ID, m.OwnerID,
		m.Name, m.PetType, m.Breed,
		m.WeightKg, m.AgeMonths,
		m.Allergies, m.SpecialNeeds, m.Notes,
		m.PhotoURL, m.VaccinationStatus,
		petDomain.PetStatus(m.Status),
		m.Version,
		m.CreatedAt, m.UpdatedAt,
	)
}
