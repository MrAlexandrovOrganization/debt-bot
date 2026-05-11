CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_identities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    platform    TEXT NOT NULL,
    external_id TEXT NOT NULL,
    UNIQUE(platform, external_id)
);

CREATE TABLE IF NOT EXISTS deals (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title      TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS deal_participants (
    deal_id UUID NOT NULL REFERENCES deals(id),
    user_id UUID NOT NULL REFERENCES users(id),
    PRIMARY KEY (deal_id, user_id)
);

CREATE TABLE IF NOT EXISTS deal_coverage (
    deal_id    UUID NOT NULL REFERENCES deals(id),
    payer_id   UUID NOT NULL REFERENCES users(id),
    covered_id UUID NOT NULL REFERENCES users(id),
    PRIMARY KEY (deal_id, covered_id)
);

CREATE TABLE IF NOT EXISTS purchases (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_id     UUID NOT NULL REFERENCES deals(id),
    title       TEXT NOT NULL,
    amount      BIGINT NOT NULL,
    paid_by     UUID NOT NULL REFERENCES users(id),
    split_mode  TEXT NOT NULL DEFAULT 'all',
    payer_share BIGINT
);

ALTER TABLE purchases ADD COLUMN IF NOT EXISTS payer_share BIGINT;

CREATE TABLE IF NOT EXISTS purchase_participants (
    purchase_id UUID NOT NULL REFERENCES purchases(id),
    user_id     UUID NOT NULL REFERENCES users(id),
    amount      BIGINT,
    PRIMARY KEY (purchase_id, user_id)
);

ALTER TABLE purchase_participants ADD COLUMN IF NOT EXISTS amount BIGINT;

CREATE TABLE IF NOT EXISTS payments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_id      UUID NOT NULL REFERENCES deals(id),
    from_user_id UUID NOT NULL REFERENCES users(id),
    to_user_id   UUID NOT NULL REFERENCES users(id),
    amount       BIGINT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
