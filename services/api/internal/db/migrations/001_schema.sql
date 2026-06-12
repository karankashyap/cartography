-- +goose Up

CREATE TABLE stores (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  platform    TEXT NOT NULL CHECK (platform IN ('SHOPIFY','AMAZON','WOOCOMMERCE')),
  imported_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE products (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id     UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  external_id  TEXT NOT NULL,
  title        TEXT NOT NULL,
  description  TEXT,
  product_type TEXT,
  vendor       TEXT,
  tags         TEXT[],
  status       TEXT,
  created_at   TIMESTAMPTZ,
  embedding    vector(768),
  UNIQUE(store_id, external_id)
);

CREATE TABLE variants (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id    UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  sku           TEXT,
  option_color  TEXT,
  option_size   TEXT,
  option_other  TEXT,
  price_cents   BIGINT NOT NULL DEFAULT 0,
  cost_cents    BIGINT,
  inventory_qty INT NOT NULL DEFAULT 0
);

CREATE TABLE customers (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id          UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  external_id       TEXT NOT NULL,
  email_hash        TEXT NOT NULL,
  country           TEXT,
  first_order_at    TIMESTAMPTZ,
  orders_count      INT NOT NULL DEFAULT 0,
  total_spent_cents BIGINT NOT NULL DEFAULT 0,
  UNIQUE(store_id, external_id)
);

CREATE TABLE orders (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id       UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  external_id    TEXT NOT NULL,
  customer_id    UUID REFERENCES customers(id),
  ordered_at     TIMESTAMPTZ NOT NULL,
  subtotal_cents BIGINT NOT NULL DEFAULT 0,
  total_cents    BIGINT NOT NULL DEFAULT 0,
  currency       TEXT NOT NULL DEFAULT 'USD',
  country        TEXT,
  is_returning   BOOL NOT NULL DEFAULT false,
  UNIQUE(store_id, external_id)
);

CREATE TABLE order_items (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id         UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  variant_id       UUID REFERENCES variants(id),
  quantity         INT NOT NULL DEFAULT 1,
  unit_price_cents BIGINT NOT NULL DEFAULT 0,
  line_total_cents BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE import_jobs (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id     UUID REFERENCES stores(id),
  filename     TEXT NOT NULL,
  platform     TEXT NOT NULL,
  state        TEXT NOT NULL DEFAULT 'pending'
               CHECK (state IN ('pending','running','done','failed')),
  rows_parsed  INT NOT NULL DEFAULT 0,
  rows_skipped INT NOT NULL DEFAULT 0,
  warnings     JSONB,
  started_at   TIMESTAMPTZ,
  finished_at  TIMESTAMPTZ,
  error        TEXT
);

CREATE TABLE chat_queries (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  store_id      UUID REFERENCES stores(id),
  question      TEXT NOT NULL,
  generated_sql TEXT NOT NULL,
  was_blocked   BOOL NOT NULL DEFAULT false,
  rows_returned INT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_store_date   ON orders(store_id, ordered_at);
CREATE INDEX idx_order_items_variant ON order_items(variant_id);
CREATE INDEX idx_variants_sku        ON variants(sku);
CREATE INDEX idx_products_store      ON products(store_id);
CREATE INDEX idx_customers_store     ON customers(store_id);
CREATE INDEX idx_import_jobs_state   ON import_jobs(state);
CREATE INDEX idx_import_jobs_store   ON import_jobs(store_id);
CREATE INDEX idx_orders_customer     ON orders(customer_id);

-- +goose Down
DROP TABLE IF EXISTS chat_queries, import_jobs, order_items, orders,
  customers, variants, products, stores CASCADE;
