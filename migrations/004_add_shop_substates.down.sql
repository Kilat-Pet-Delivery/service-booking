DROP INDEX IF EXISTS idx_bookings_qr_pickup_token;
DROP INDEX IF EXISTS idx_bookings_shop_id;
ALTER TABLE bookings DROP COLUMN IF EXISTS qr_pickup_token;
ALTER TABLE bookings DROP COLUMN IF EXISTS shop_id;
