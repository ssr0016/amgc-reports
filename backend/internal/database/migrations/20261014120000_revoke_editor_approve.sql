-- +goose Up
-- +goose StatementBegin
-- Only the admin approves reports. The seed only ever adds grants (ON CONFLICT DO
-- NOTHING), so a database seeded before this change still has the editor grant
-- until it is deleted here. Reports already approved keep their approval.
DELETE FROM role_permissions rp
USING roles r, permissions p
WHERE rp.role_id = r.id AND r.name = 'editor'
  AND rp.permission_id = p.id AND p.name = 'reports:approve';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'editor' AND p.name = 'reports:approve'
ON CONFLICT (role_id, permission_id) DO NOTHING;
-- +goose StatementEnd
