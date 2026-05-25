DROP INDEX IF EXISTS idx_bookings_runner_scheduled;
DROP INDEX IF EXISTS idx_bookings_owner_scheduled;
DROP INDEX IF EXISTS idx_bookings_scheduled_at;

ALTER TABLE pets
    DROP COLUMN IF EXISTS vet_contact_phone,
    DROP COLUMN IF EXISTS vet_contact_name,
    DROP COLUMN IF EXISTS temperament;
