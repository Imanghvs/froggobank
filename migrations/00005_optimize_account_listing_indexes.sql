-- +goose Up
CREATE INDEX account_owner_created_at_id_idx
ON accounts (owner_id, created_at DESC, id DESC)
WHERE owner_id IS NOT NULL;

-- +goose Down
DROP INDEX account_owner_created_at_id_idx;
