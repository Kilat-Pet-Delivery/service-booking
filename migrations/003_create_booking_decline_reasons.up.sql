-- 003_create_booking_decline_reasons.sql
-- Tracks every decline for ops analytics. Future: route past-declined jobs differently.

CREATE TABLE IF NOT EXISTS booking_decline_reasons (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    booking_id  UUID        NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    runner_id   UUID        NOT NULL,
    reason      TEXT        NOT NULL,
    declined_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for looking up declines by booking
CREATE INDEX IF NOT EXISTS idx_decline_booking_id ON booking_decline_reasons(booking_id);
CREATE INDEX IF NOT EXISTS idx_decline_runner_id ON booking_decline_reasons(runner_id);
