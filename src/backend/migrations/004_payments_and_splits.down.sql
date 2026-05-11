ALTER TABLE purchase_participants DROP COLUMN IF EXISTS amount;

ALTER TABLE purchases DROP COLUMN IF EXISTS payer_share;

DROP TABLE IF EXISTS payments;
