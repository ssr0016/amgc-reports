-- +goose Up
-- +goose StatementBegin
-- Profile picture (Feature 33): the public R2 URL of the user's avatar; NULL shows initials.
ALTER TABLE users
    ADD COLUMN avatar_url TEXT NULL CONSTRAINT chk_users_avatar_url_length CHECK (char_length(avatar_url) <= 500);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS avatar_url;
-- +goose StatementEnd
