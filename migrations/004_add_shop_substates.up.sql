ALTER TABLE bookings ADD COLUMN IF NOT EXISTS shop_id UUID;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS qr_pickup_token VARCHAR(32);

CREATE INDEX IF NOT EXISTS idx_bookings_shop_id ON bookings(shop_id);
CREATE INDEX IF NOT EXISTS idx_bookings_qr_pickup_token ON bookings(qr_pickup_token);
