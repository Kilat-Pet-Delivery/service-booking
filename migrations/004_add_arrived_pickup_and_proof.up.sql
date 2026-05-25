ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS arrived_at_pickup TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS proofs_of_delivery (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    booking_id UUID NOT NULL UNIQUE REFERENCES bookings(id) ON DELETE CASCADE,
    photo_url TEXT NOT NULL,
    signature_url TEXT NOT NULL,
    recipient_kind VARCHAR(30) NOT NULL CHECK (recipient_kind IN ('customer', 'receptionist', 'left_at_door')),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proofs_of_delivery_booking_id ON proofs_of_delivery(booking_id);
