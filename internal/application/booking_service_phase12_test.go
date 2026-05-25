package application

import (
	"context"
	"testing"
	"time"

	commonDomain "github.com/Kilat-Pet-Delivery/lib-common/domain"
	"github.com/Kilat-Pet-Delivery/lib-proto/dto"
	bookingDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/booking"
	photoDomain "github.com/Kilat-Pet-Delivery/service-booking/internal/domain/photo"
	"github.com/google/uuid"
)

type phase12BookingRepo struct {
	bookings map[uuid.UUID]*bookingDomain.Booking
}

func newPhase12BookingRepo(bookings ...*bookingDomain.Booking) *phase12BookingRepo {
	repo := &phase12BookingRepo{bookings: map[uuid.UUID]*bookingDomain.Booking{}}
	for _, booking := range bookings {
		repo.bookings[booking.ID()] = booking
	}
	return repo
}

func (r *phase12BookingRepo) FindByID(_ context.Context, id uuid.UUID) (*bookingDomain.Booking, error) {
	booking, ok := r.bookings[id]
	if !ok {
		return nil, commonDomain.ErrNotFound
	}
	return booking, nil
}

func (r *phase12BookingRepo) FindByNumber(context.Context, string) (*bookingDomain.Booking, error) {
	return nil, commonDomain.ErrNotFound
}

func (r *phase12BookingRepo) FindByOwnerID(context.Context, uuid.UUID, int, int) ([]*bookingDomain.Booking, int64, error) {
	return nil, 0, nil
}

func (r *phase12BookingRepo) FindByRunnerID(context.Context, uuid.UUID, int, int) ([]*bookingDomain.Booking, int64, error) {
	return nil, 0, nil
}

func (r *phase12BookingRepo) FindHistoryByOwnerID(context.Context, uuid.UUID, int, int) ([]*bookingDomain.Booking, int64, error) {
	return nil, 0, nil
}

func (r *phase12BookingRepo) FindHistoryByRunnerID(context.Context, uuid.UUID, int, int) ([]*bookingDomain.Booking, int64, error) {
	return nil, 0, nil
}

func (r *phase12BookingRepo) FindScheduledByOwnerID(context.Context, uuid.UUID, time.Time, int, int) ([]*bookingDomain.Booking, int64, error) {
	return nil, 0, nil
}

func (r *phase12BookingRepo) FindScheduledByRunnerID(context.Context, uuid.UUID, time.Time, int, int) ([]*bookingDomain.Booking, int64, error) {
	return nil, 0, nil
}

func (r *phase12BookingRepo) ListAll(context.Context, int, int) ([]*bookingDomain.Booking, int64, error) {
	return nil, 0, nil
}

func (r *phase12BookingRepo) CountByStatus(context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func (r *phase12BookingRepo) Save(_ context.Context, booking *bookingDomain.Booking) error {
	r.bookings[booking.ID()] = booking
	return nil
}

func (r *phase12BookingRepo) Update(_ context.Context, booking *bookingDomain.Booking) error {
	r.bookings[booking.ID()] = booking
	return nil
}

type phase12ProofRepo struct {
	proofs map[uuid.UUID]*bookingDomain.ProofOfDelivery
}

func (r *phase12ProofRepo) Save(_ context.Context, proof *bookingDomain.ProofOfDelivery) error {
	r.proofs[proof.BookingID()] = proof
	return nil
}

func (r *phase12ProofRepo) FindByBookingID(_ context.Context, bookingID uuid.UUID) (*bookingDomain.ProofOfDelivery, error) {
	proof, ok := r.proofs[bookingID]
	if !ok {
		return nil, commonDomain.ErrNotFound
	}
	return proof, nil
}

type phase12PhotoRepo struct {
	photos []*photoDomain.BookingPhoto
}

func (r *phase12PhotoRepo) Save(_ context.Context, photo *photoDomain.BookingPhoto) error {
	r.photos = append(r.photos, photo)
	return nil
}

func (r *phase12PhotoRepo) FindByBookingID(context.Context, uuid.UUID) ([]*photoDomain.BookingPhoto, error) {
	return r.photos, nil
}

func (r *phase12PhotoRepo) FindByID(context.Context, uuid.UUID) (*photoDomain.BookingPhoto, error) {
	return nil, commonDomain.ErrNotFound
}

func Test_ArriveAtPickup_TransitionsStatus_FromAcceptedToPickupArrived(t *testing.T) {
	booking := acceptedBooking(t)
	service := &BookingService{repo: newPhase12BookingRepo(booking)}

	result, err := service.ArriveAtPickup(context.Background(), booking.ID())
	if err != nil {
		t.Fatalf("ArriveAtPickup returned error: %v", err)
	}
	if result.Status != string(bookingDomain.StatusPickupArrived) {
		t.Fatalf("expected pickup_arrived, got %s", result.Status)
	}
	if result.ArrivedAtPickup == nil {
		t.Fatalf("expected arrived_at_pickup timestamp")
	}
}

func Test_ArriveAtPickup_RejectsFromWrongState(t *testing.T) {
	booking := newBooking(t)
	service := &BookingService{repo: newPhase12BookingRepo(booking)}

	if _, err := service.ArriveAtPickup(context.Background(), booking.ID()); err == nil {
		t.Fatalf("expected invalid transition error")
	}
}

func Test_SubmitProofOfDelivery_PersistsTwoPhotos(t *testing.T) {
	booking := deliveredBooking(t)
	proofRepo := &phase12ProofRepo{proofs: map[uuid.UUID]*bookingDomain.ProofOfDelivery{}}
	photoRepo := &phase12PhotoRepo{}
	service := &BookingService{
		repo:      newPhase12BookingRepo(booking),
		proofRepo: proofRepo,
		photoRepo: photoRepo,
	}

	result, err := service.SubmitProofOfDelivery(context.Background(), booking.ID(), *booking.RunnerID(), ProofOfDeliveryRequest{
		PhotoURL:      "memory://proof.jpg",
		SignatureURL:  "memory://signature.png",
		RecipientKind: bookingDomain.RecipientCustomer,
	})
	if err != nil {
		t.Fatalf("SubmitProofOfDelivery returned error: %v", err)
	}
	if result.RecipientKind != bookingDomain.RecipientCustomer {
		t.Fatalf("expected recipient kind customer, got %s", result.RecipientKind)
	}
	if len(photoRepo.photos) != 2 {
		t.Fatalf("expected proof and signature photos, got %d", len(photoRepo.photos))
	}
}

func Test_CompleteDelivery_IsIdempotent(t *testing.T) {
	booking := deliveredBooking(t)
	service := &BookingService{repo: newPhase12BookingRepo(booking)}

	if _, err := service.CompleteDelivery(context.Background(), booking.ID()); err != nil {
		t.Fatalf("first CompleteDelivery returned error: %v", err)
	}
	if _, err := service.CompleteDelivery(context.Background(), booking.ID()); err != nil {
		t.Fatalf("second CompleteDelivery returned error: %v", err)
	}
}

func Test_CancelActiveDelivery_CancelsInProgressBooking(t *testing.T) {
	booking := inProgressBooking(t)
	service := &BookingService{repo: newPhase12BookingRepo(booking)}

	result, err := service.CancelActiveDelivery(context.Background(), booking.ID(), uuid.New(), "customer cancelled")
	if err != nil {
		t.Fatalf("CancelActiveDelivery returned error: %v", err)
	}
	if result.Status != string(bookingDomain.StatusCancelled) {
		t.Fatalf("expected cancelled, got %s", result.Status)
	}
}

func newBooking(t *testing.T) *bookingDomain.Booking {
	t.Helper()
	booking, err := bookingDomain.NewBooking(
		uuid.New(),
		bookingDomain.PetSpecification{PetType: "cat", Name: "Miso", WeightKg: 4},
		bookingDomain.CrateRequirement{MinimumSize: "small", NeedsVentilation: true, MinimumWeightCapacity: 5},
		dto.AddressDTO{Line1: "pickup", City: "KL", State: "WP", Country: "MY", Latitude: 3.13, Longitude: 101.68},
		dto.AddressDTO{Line1: "dropoff", City: "KL", State: "WP", Country: "MY", Latitude: 3.14, Longitude: 101.69},
		1000,
		"MYR",
		nil,
		"",
	)
	if err != nil {
		t.Fatalf("NewBooking returned error: %v", err)
	}
	return booking
}

func acceptedBooking(t *testing.T) *bookingDomain.Booking {
	t.Helper()
	booking := newBooking(t)
	if err := booking.Accept(uuid.New()); err != nil {
		t.Fatalf("Accept returned error: %v", err)
	}
	booking.IncrementVersion()
	return booking
}

func inProgressBooking(t *testing.T) *bookingDomain.Booking {
	t.Helper()
	booking := acceptedBooking(t)
	if err := booking.ArriveAtPickup(); err != nil {
		t.Fatalf("ArriveAtPickup returned error: %v", err)
	}
	booking.IncrementVersion()
	if err := booking.StartDelivery(); err != nil {
		t.Fatalf("StartDelivery returned error: %v", err)
	}
	booking.IncrementVersion()
	return booking
}

func deliveredBooking(t *testing.T) *bookingDomain.Booking {
	t.Helper()
	booking := inProgressBooking(t)
	if err := booking.ConfirmDelivery(); err != nil {
		t.Fatalf("ConfirmDelivery returned error: %v", err)
	}
	booking.IncrementVersion()
	return booking
}
