package booking

import "fmt"

// BookingStatus represents the current state of a booking in its lifecycle.
type BookingStatus string

const (
	StatusRequested  BookingStatus = "requested"
	StatusAccepted   BookingStatus = "accepted"
	StatusInProgress BookingStatus = "in_progress"
	StatusDelivered  BookingStatus = "delivered"
	StatusCompleted  BookingStatus = "completed"
	StatusCancelled  BookingStatus = "cancelled"
)

// validTransitions defines the state machine for booking status transitions.
var validTransitions = map[BookingStatus][]BookingStatus{
	StatusRequested:  {StatusAccepted, StatusCancelled},
	StatusAccepted:   {StatusInProgress, StatusCancelled},
	StatusInProgress: {StatusDelivered, StatusCancelled},
	StatusDelivered:  {StatusCompleted},
	StatusCompleted:  {},
	StatusCancelled:  {},
}

// IsValid returns true if the status is a recognized booking status.
func (s BookingStatus) IsValid() bool {
	_, exists := validTransitions[s]
	return exists
}

// CanTransitionTo returns true if a transition from this status to the target is allowed.
func (s BookingStatus) CanTransitionTo(target BookingStatus) bool {
	allowed, exists := validTransitions[s]
	if !exists {
		return false
	}
	for _, t := range allowed {
		if t == target {
			return true
		}
	}
	return false
}

// IsTerminal returns true if no further transitions are possible from this status.
func (s BookingStatus) IsTerminal() bool {
	allowed, exists := validTransitions[s]
	if !exists {
		return true
	}
	return len(allowed) == 0
}

// CanBeCancelled returns true if the booking can be cancelled from this status.
func (s BookingStatus) CanBeCancelled() bool {
	return s.CanTransitionTo(StatusCancelled)
}

// String returns the string representation of the status.
func (s BookingStatus) String() string {
	return string(s)
}

// ParseBookingStatus converts a string to a BookingStatus, returning an error if invalid.
func ParseBookingStatus(s string) (BookingStatus, error) {
	status := BookingStatus(s)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid booking status: %s", s)
	}
	return status, nil
}
