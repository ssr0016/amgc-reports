// The four account statuses (D-A10), in the order the filter and the Edit page list them.
// The server owns the rules; this is only the wording. "Deleted" is not a status: the Deleted filter is a separate server query (status=deleted).
export const USER_STATUSES = [
  "active",
  "inactive",
  "disciplinary",
  "deceased",
] as const;
export type UserStatus = (typeof USER_STATUSES)[number];

export const STATUS_LABEL: Record<UserStatus, string> = {
  active: "Active",
  inactive: "Inactive",
  disciplinary: "Disciplinary",
  deceased: "Deceased",
};

// A value this code does not know is shown as it came, not hidden.
export const statusLabel = (status: string) =>
  STATUS_LABEL[status as UserStatus] ?? status;
