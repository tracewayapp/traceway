CREATE TABLE products (
  id               bigserial PRIMARY KEY,
  category_id      bigint NOT NULL REFERENCES categories (id),
  brand_id         bigint REFERENCES brands (id),
  name             text NOT NULL,
  slug             text NOT NULL UNIQUE,
  price_cents      integer NOT NULL,
  compare_at_cents integer,
  currency         char(3) NOT NULL DEFAULT 'EUR',
  thumbnail_url    text,
  description_html text,
  spec_sheet       jsonb,
  sort_rank        integer NOT NULL DEFAULT 0,
  is_active        boolean NOT NULL DEFAULT true,
  search_vector    tsvector,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX products_category_rank_idx ON products (category_id, sort_rank DESC) WHERE is_active;
CREATE INDEX products_search_idx ON products USING gin (search_vector);
