package booking

import "fmt"

// PricingStrategy defines the interface for calculating booking prices.
type PricingStrategy interface {
	// Calculate returns the estimated price in cents for the given parameters.
	Calculate(params PricingParams) (int64, error)
}

// PricingParams holds the inputs for price calculation.
type PricingParams struct {
	DistanceKm  float64
	PetType     PetType
	CrateSize   CrateSize
	IsScheduled bool
}

// StandardPricingStrategy implements the default pricing logic for Kilat Pet Runner.
type StandardPricingStrategy struct{}

// NewStandardPricingStrategy creates a new StandardPricingStrategy.
func NewStandardPricingStrategy() *StandardPricingStrategy {
	return &StandardPricingStrategy{}
}

// Calculate computes the estimated price in cents (sen for MYR).
//
// Pricing formula:
//   - Base fare: MYR 5.00 (500 sen)
//   - Distance: MYR 2.50/km (250 sen/km)
//   - Pet surcharge: varies by pet type
//   - Crate surcharge: varies by crate size
func (s *StandardPricingStrategy) Calculate(params PricingParams) (int64, error) {
	if params.DistanceKm < 0 {
		return 0, fmt.Errorf("distance cannot be negative")
	}

	// Base fare: MYR 5.00
	var totalCents int64 = 500

	// Distance charge: MYR 2.50 per km
	totalCents += int64(params.DistanceKm * 250)

	// Pet type surcharge
	petSurcharge, err := petTypeSurcharge(params.PetType)
	if err != nil {
		return 0, err
	}
	totalCents += petSurcharge

	// Crate size surcharge
	totalCents += crateSizeSurcharge(params.CrateSize)

	return totalCents, nil
}

// petTypeSurcharge returns the surcharge in cents based on pet type.
func petTypeSurcharge(petType PetType) (int64, error) {
	switch petType {
	case PetTypeDog:
		return 500, nil // MYR 5.00
	case PetTypeCat:
		return 300, nil // MYR 3.00
	case PetTypeBird:
		return 200, nil // MYR 2.00
	case PetTypeReptile:
		return 800, nil // MYR 8.00
	case PetTypeRabbit:
		return 300, nil // MYR 3.00
	case PetTypeOther:
		return 500, nil // MYR 5.00
	default:
		return 0, fmt.Errorf("unknown pet type for pricing: %s", petType)
	}
}

// crateSizeSurcharge returns the surcharge in cents based on crate size.
func crateSizeSurcharge(size CrateSize) int64 {
	switch size {
	case CrateSizeSmall:
		return 0
	case CrateSizeMedium:
		return 500 // MYR 5.00
	case CrateSizeLarge:
		return 1000 // MYR 10.00
	case CrateSizeXLarge:
		return 2000 // MYR 20.00
	default:
		return 0
	}
}
