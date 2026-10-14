// Names copied from backend/internal/database/seed.go (not generated: Swagger
// does not list them). Keep in sync when the seed changes. The server still
// enforces every permission; the UI only uses these to show or hide controls.
export type Permission =
  | "users:read"
  | "users:write"
  | "users:delete"
  | "roles:read"
  | "roles:write"
  | "roles:delete"
  | "permissions:read"
  | "permissions:write"
  | "permissions:delete"
  | "reports:read"
  | "reports:read_any"
  | "reports:write"
  | "reports:write_any"
  | "reports:delete"
  | "reports:delete_any"
  | "reports:approve";
