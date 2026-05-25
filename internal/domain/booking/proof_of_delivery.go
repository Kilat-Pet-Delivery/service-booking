package booking

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	RecipientCustomer     = "customer"
	RecipientReceptionist = "receptionist"
	RecipientLeftAtDoor   = "left_at_door"
)

// ProofOfDelivery records the final handoff evidence for a booking.
type ProofOfDelivery struct {
	id            uuid.UUID
	bookingID     uuid.UUID
	photoURL      string
	signatureURL  string
	recipientKind string
	notes         string
	createdAt     time.Time
}

// NewProofOfDelivery creates proof evidence for a booking.
func NewProofOfDelivery(bookingID uuid.UUID, photoURL, signatureURL, recipientKind, notes string) (*ProofOfDelivery, error) {
	if bookingID == uuid.Nil {
		return nil, fmt.Errorf("booking ID is required")
	}
	if photoURL == "" {
		return nil, fmt.Errorf("proof photo URL is required")
	}
	if signatureURL == "" {
		return nil, fmt.Errorf("signature URL is required")
	}
	if !validRecipientKind(recipientKind) {
		return nil, fmt.Errorf("recipient kind must be one of: customer, receptionist, left_at_door")
	}
	return &ProofOfDelivery{
		id:            uuid.New(),
		bookingID:     bookingID,
		photoURL:      photoURL,
		signatureURL:  signatureURL,
		recipientKind: recipientKind,
		notes:         notes,
		createdAt:     time.Now().UTC(),
	}, nil
}

// ReconstructProofOfDelivery rebuilds proof evidence from persistence data.
func ReconstructProofOfDelivery(id, bookingID uuid.UUID, photoURL, signatureURL, recipientKind, notes string, createdAt time.Time) *ProofOfDelivery {
	return &ProofOfDelivery{
		id:            id,
		bookingID:     bookingID,
		photoURL:      photoURL,
		signatureURL:  signatureURL,
		recipientKind: recipientKind,
		notes:         notes,
		createdAt:     createdAt,
	}
}

func (p *ProofOfDelivery) ID() uuid.UUID         { return p.id }
func (p *ProofOfDelivery) BookingID() uuid.UUID  { return p.bookingID }
func (p *ProofOfDelivery) PhotoURL() string      { return p.photoURL }
func (p *ProofOfDelivery) SignatureURL() string  { return p.signatureURL }
func (p *ProofOfDelivery) RecipientKind() string { return p.recipientKind }
func (p *ProofOfDelivery) Notes() string         { return p.notes }
func (p *ProofOfDelivery) CreatedAt() time.Time  { return p.createdAt }

func validRecipientKind(kind string) bool {
	return kind == RecipientCustomer || kind == RecipientReceptionist || kind == RecipientLeftAtDoor
}
