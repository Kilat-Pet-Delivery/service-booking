package booking

// RouteSpecification is a value object representing the calculated route between pickup and dropoff.
type RouteSpecification struct {
	PickupLat           float64 `json:"pickup_lat"`
	PickupLng           float64 `json:"pickup_lng"`
	DropoffLat          float64 `json:"dropoff_lat"`
	DropoffLng          float64 `json:"dropoff_lng"`
	DistanceKm          float64 `json:"distance_km"`
	EstimatedDurationMin int    `json:"estimated_duration_min"`
	Polyline            string  `json:"polyline"`
}
