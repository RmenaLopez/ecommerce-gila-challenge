CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status TEXT NOT NULL DEFAULT 'pending',
    total_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products (id),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0)
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);
