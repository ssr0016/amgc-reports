import type { Permission } from "./permissions";

// Shared by the sidebar and panel so their permission gates and fallback order agree.
export function adminSections(can: (permission: Permission) => boolean, isAdmin: boolean) {
  return [
    { value: "reports", label: "Reports", allowed: can("reports:approve") || can("reports:export") },
    { value: "tracker", label: "Tracker", allowed: can("tracker:read") },
    { value: "users", label: "Users", allowed: can("users:list") },
    { value: "roles", label: "Roles", allowed: isAdmin },
    { value: "logs", label: "Logs", allowed: can("audit:read") },
  ].filter((section) => section.allowed);
}
