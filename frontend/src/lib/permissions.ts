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
  | "reports:approve"
  | "reports:export"
  | "tracker:read"
  | "audit:read"
  | "users:list";

// Any one of these opens /admin (37b); each opens its own tab. Keep in step with the tabs in admin-panel.tsx.
export const ADMIN_AREA: Permission[] = ["reports:approve", "reports:export", "tracker:read", "users:list", "audit:read"];
