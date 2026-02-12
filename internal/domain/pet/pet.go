package pet

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PetStatus represents the lifecycle state of a pet profile.
type PetStatus string

const (
	PetStatusActive   PetStatus = "active"
	PetStatusArchived PetStatus = "archived"
)

// Pet is the aggregate root for a saved pet profile.
type Pet struct {
	id                uuid.UUID
	ownerID           uuid.UUID
	name              string
	petType           string
	breed             string
	weightKg          float64
	ageMonths         int
	allergies         string
	specialNeeds      string
	notes             string
	photoURL          string
	vaccinationStatus string
	status            PetStatus
	version           int64
	createdAt         time.Time
	updatedAt         time.Time
}

// NewPet creates a new active pet profile with validated fields.
func NewPet(
	ownerID uuid.UUID,
	name, petType, breed string,
	weightKg float64,
	ageMonths int,
	allergies, specialNeeds, notes, photoURL, vaccinationStatus string,
) (*Pet, error) {
	if ownerID == uuid.Nil {
		return nil, fmt.Errorf("owner ID is required")
	}
	if name == "" {
		return nil, fmt.Errorf("pet name is required")
	}
	if petType == "" {
		return nil, fmt.Errorf("pet type is required")
	}

	now := time.Now().UTC()
	return &Pet{
		id:                uuid.New(),
		ownerID:           ownerID,
		name:              name,
		petType:           petType,
		breed:             breed,
		weightKg:          weightKg,
		ageMonths:         ageMonths,
		allergies:         allergies,
		specialNeeds:      specialNeeds,
		notes:             notes,
		photoURL:          photoURL,
		vaccinationStatus: vaccinationStatus,
		status:            PetStatusActive,
		version:           1,
		createdAt:         now,
		updatedAt:         now,
	}, nil
}

// Reconstruct rebuilds a Pet from persistence data (no validation).
func Reconstruct(
	id, ownerID uuid.UUID,
	name, petType, breed string,
	weightKg float64,
	ageMonths int,
	allergies, specialNeeds, notes, photoURL, vaccinationStatus string,
	status PetStatus,
	version int64,
	createdAt, updatedAt time.Time,
) *Pet {
	return &Pet{
		id:                id,
		ownerID:           ownerID,
		name:              name,
		petType:           petType,
		breed:             breed,
		weightKg:          weightKg,
		ageMonths:         ageMonths,
		allergies:         allergies,
		specialNeeds:      specialNeeds,
		notes:             notes,
		photoURL:          photoURL,
		vaccinationStatus: vaccinationStatus,
		status:            status,
		version:           version,
		createdAt:         createdAt,
		updatedAt:         updatedAt,
	}
}

// --- Getters ---

func (p *Pet) ID() uuid.UUID          { return p.id }
func (p *Pet) OwnerID() uuid.UUID     { return p.ownerID }
func (p *Pet) Name() string           { return p.name }
func (p *Pet) PetType() string        { return p.petType }
func (p *Pet) Breed() string          { return p.breed }
func (p *Pet) WeightKg() float64      { return p.weightKg }
func (p *Pet) AgeMonths() int         { return p.ageMonths }
func (p *Pet) Allergies() string      { return p.allergies }
func (p *Pet) SpecialNeeds() string   { return p.specialNeeds }
func (p *Pet) Notes() string          { return p.notes }
func (p *Pet) PhotoURL() string       { return p.photoURL }
func (p *Pet) VaccinationStatus() string { return p.vaccinationStatus }
func (p *Pet) Status() PetStatus      { return p.status }
func (p *Pet) Version() int64         { return p.version }
func (p *Pet) CreatedAt() time.Time   { return p.createdAt }
func (p *Pet) UpdatedAt() time.Time   { return p.updatedAt }

// --- Behavior ---

// IsOwnedBy checks if the pet belongs to the given owner.
func (p *Pet) IsOwnedBy(ownerID uuid.UUID) bool {
	return p.ownerID == ownerID
}

// Update applies partial updates to the pet profile.
func (p *Pet) Update(
	name, petType, breed string,
	weightKg float64,
	ageMonths int,
	allergies, specialNeeds, notes, photoURL, vaccinationStatus string,
) {
	if name != "" {
		p.name = name
	}
	if petType != "" {
		p.petType = petType
	}
	if breed != "" {
		p.breed = breed
	}
	if weightKg > 0 {
		p.weightKg = weightKg
	}
	if ageMonths > 0 {
		p.ageMonths = ageMonths
	}
	if allergies != "" {
		p.allergies = allergies
	}
	if specialNeeds != "" {
		p.specialNeeds = specialNeeds
	}
	if notes != "" {
		p.notes = notes
	}
	if photoURL != "" {
		p.photoURL = photoURL
	}
	if vaccinationStatus != "" {
		p.vaccinationStatus = vaccinationStatus
	}
	p.version++
	p.updatedAt = time.Now().UTC()
}

// Archive marks the pet profile as archived.
func (p *Pet) Archive() {
	p.status = PetStatusArchived
	p.version++
	p.updatedAt = time.Now().UTC()
}

// IsActive returns true if the pet profile is active.
func (p *Pet) IsActive() bool {
	return p.status == PetStatusActive
}
