CREATE TABLE IF NOT EXISTS booking_items (
    booking_id      UUID NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    product_id      UUID NOT NULL,
    qty             BIGINT NOT NULL,
    price_myr_cents BIGINT NOT NULL,
    sku             VARCHAR(100),
    name            VARCHAR(255),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (booking_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_booking_items_booking_id ON booking_items(booking_id);
