ALTER TABLE pets
    ADD COLUMN IF NOT EXISTS temperament JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS vet_contact_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS vet_contact_phone VARCHAR(30);

CREATE INDEX IF NOT EXISTS idx_bookings_scheduled_at ON bookings(scheduled_at);
CREATE INDEX IF NOT EXISTS idx_bookings_owner_scheduled ON bookings(owner_id, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_bookings_runner_scheduled ON bookings(runner_id, scheduled_at);
