CREATE TABLE payments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_id      UUID NOT NULL REFERENCES deals(id),
    from_user_id UUID NOT NULL REFERENCES users(id),
    to_user_id   UUID NOT NULL REFERENCES users(id),
    amount       BIGINT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE purchases ADD COLUMN payer_share BIGINT;

ALTER TABLE purchase_participants ADD COLUMN amount BIGINT;
