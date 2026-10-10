-- +goose Up
-- +goose StatementBegin

-- 1. Insert default 'user' role if not exists
INSERT INTO roles (name, description)
VALUES ('user', 'Default user role')
ON CONFLICT (name) DO NOTHING;

-- 2. Add role_id column (nullable muna)
ALTER TABLE users ADD COLUMN role_id BIGINT;

-- 3. Backfill: assign 'user' role to existing users
UPDATE users
SET role_id = (SELECT id FROM roles WHERE name = 'user')
WHERE role_id IS NULL;

-- 4. Make role_id NOT NULL
ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;

-- 5. Add FK constraint
ALTER TABLE users
ADD CONSTRAINT fk_users_role
FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE RESTRICT;

-- 6. Add index
CREATE INDEX idx_users_role_id ON users(role_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_users_role_id;
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_role;
ALTER TABLE users DROP COLUMN IF EXISTS role_id;
DELETE FROM roles WHERE name = 'user';
-- +goose StatementEnd
