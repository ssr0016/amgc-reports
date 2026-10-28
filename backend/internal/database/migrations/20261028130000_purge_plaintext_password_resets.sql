-- +goose Up
-- Reset tokens are now stored as token.Hash (M3). Drop the plaintext rows; each
-- lives an hour at most, so this only voids resets requested just before the deploy.
DELETE FROM password_resets;

-- +goose Down
-- The plaintext tokens cannot be restored.
SELECT 1;
