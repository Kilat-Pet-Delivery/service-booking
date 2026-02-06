package booking

import "time"

// PetType represents the type of pet being transported.
type PetType string

const (
	PetTypeCat     PetType = "cat"
	PetTypeDog     PetType = "dog"
	PetTypeBird    PetType = "bird"
	PetTypeRabbit  PetType = "rabbit"
	PetTypeReptile PetType = "reptile"
	PetTypeOther   PetType = "other"
)

// IsValid returns true if the pet type is recognized.
func (p PetType) IsValid() bool {
	switch p {
	case PetTypeCat, PetTypeDog, PetTypeBird, PetTypeRabbit, PetTypeReptile, PetTypeOther:
		return true
	}
	return false
}

// CrateSize represents the size of the crate required for transport.
type CrateSize string

const (
	CrateSizeSmall  CrateSize = "small"
	CrateSizeMedium CrateSize = "medium"
	CrateSizeLarge  CrateSize = "large"
	CrateSizeXLarge CrateSize = "xlarge"
)

// IsValid returns true if the crate size is recognized.
func (c CrateSize) IsValid() bool {
	switch c {
	case CrateSizeSmall, CrateSizeMedium, CrateSizeLarge, CrateSizeXLarge:
		return true
	}
	return false
}

// PetSpecification is an immutable value object describing the pet to be transported.
type PetSpecification struct {
	PetType      string             `json:"pet_type"`
	Breed        string             `json:"breed"`
	Name         string             `json:"name"`
	WeightKg     float64            `json:"weight_kg"`
	Age          int                `json:"age_months"`
	Vaccinations []VaccinationRecord `json:"vaccinations"`
	SpecialNeeds string             `json:"special_needs"`
	PhotoURL     string             `json:"photo_url"`
}

// VaccinationRecord represents a single vaccination entry for a pet.
type VaccinationRecord struct {
	VaccineName string     `json:"vaccine_name"`
	DateGiven   time.Time  `json:"date_given"`
	ExpiresAt   *time.Time `json:"expires_at"`
	VetName     string     `json:"vet_name"`
	Verified    bool       `json:"verified"`
}

// CrateRequirement describes the minimum crate specifications needed for a pet.
type CrateRequirement struct {
	MinimumSize            CrateSize `json:"minimum_size"`
	NeedsVentilation       bool      `json:"needs_ventilation"`
	NeedsTempControl       bool      `json:"needs_temp_control"`
	MinimumWeightCapacity  float64   `json:"minimum_weight_capacity"`
}

// DetermineCrateRequirement automatically determines the crate requirements based on pet specs.
func DetermineCrateRequirement(spec PetSpecification) CrateRequirement {
	req := CrateRequirement{
		NeedsVentilation:      true,
		NeedsTempControl:      false,
		MinimumWeightCapacity: spec.WeightKg * 1.2, // 20% buffer
	}

	// Determine crate size based on weight
	switch {
	case spec.WeightKg <= 5:
		req.MinimumSize = CrateSizeSmall
	case spec.WeightKg <= 15:
		req.MinimumSize = CrateSizeMedium
	case spec.WeightKg <= 30:
		req.MinimumSize = CrateSizeLarge
	default:
		req.MinimumSize = CrateSizeXLarge
	}

	// Reptiles need temperature control
	if PetType(spec.PetType) == PetTypeReptile {
		req.NeedsTempControl = true
	}

	return req
}
