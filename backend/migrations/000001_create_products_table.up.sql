CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL,
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    stock INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
    weight_kg NUMERIC(10, 3) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_products_category ON products (category);
CREATE INDEX idx_products_name_trgm ON products USING gin (name gin_trgm_ops);
