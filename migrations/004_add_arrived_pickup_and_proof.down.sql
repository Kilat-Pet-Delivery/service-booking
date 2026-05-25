DROP INDEX IF EXISTS idx_proofs_of_delivery_booking_id;
DROP TABLE IF EXISTS proofs_of_delivery;

ALTER TABLE bookings
    DROP COLUMN IF EXISTS arrived_at_pickup;
