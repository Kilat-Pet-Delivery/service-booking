CREATE TABLE IF NOT EXISTS bookings (
    id                    UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    booking_number        VARCHAR(20) NOT NULL UNIQUE,
    owner_id              UUID NOT NULL,
    runner_id             UUID,
    status                VARCHAR(30) NOT NULL DEFAULT 'requested',
    pet_spec              JSONB NOT NULL,
    crate_requirement     JSONB NOT NULL,
    pickup_address        JSONB NOT NULL,
    dropoff_address       JSONB NOT NULL,
    route_spec            JSONB,
    estimated_price_cents BIGINT NOT NULL,
    final_price_cents     BIGINT,
    currency              VARCHAR(3) NOT NULL DEFAULT 'MYR',
    scheduled_at          TIMESTAMPTZ,
    picked_up_at          TIMESTAMPTZ,
    delivered_at          TIMESTAMPTZ,
    cancelled_at          TIMESTAMPTZ,
    cancel_note           VARCHAR(500),
    notes                 VARCHAR(1000),
    version               BIGINT NOT NULL DEFAULT 1,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_bookings_owner_id ON bookings(owner_id);
CREATE INDEX IF NOT EXISTS idx_bookings_runner_id ON bookings(runner_id);
CREATE INDEX IF NOT EXISTS idx_bookings_status ON bookings(status);
CREATE INDEX IF NOT EXISTS idx_bookings_booking_number ON bookings(booking_number);
CREATE INDEX IF NOT EXISTS idx_bookings_created_at ON bookings(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bookings_owner_status ON bookings(owner_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_runner_status ON bookings(runner_id, status);
