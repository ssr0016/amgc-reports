import type { components } from "./api-types";

type Schemas = components["schemas"];
type Models = {
  [K in keyof Schemas as K extends `github_com_ssr0016_pastoral-reports_backend_internal_model.${infer N}`
    ? N
    : never]: Schemas[K];
};
type Model<N extends keyof Models> = Models[N];

// swag marks a response field required only when the Go field has no omitempty
// and a validate tag, so the generated types make everything optional. The
// server always sends the fields listed here, so they are made required.
type Always<T, K extends keyof T> = Required<Pick<T, K>> & Omit<T, K>;

export type Me = Required<Model<"MeResponse">>;
export type AdminStats = Required<Model<"AdminStats">>;

export type MetricWeeks = Model<"MetricWeeks">;
export type ReportInput = Model<"CreateReportRequest"> & { user_notes?: string };
export type Report = Always<
  Model<"ReportResponse">,
  "id" | "created_by" | "month" | "year" | "metrics" | "completed" | "created_at" | "updated_at" | "owner_status"
>;
export type ExcelPreview = { count: number; sheets: Required<Model<"PreviewSheet">>[] };
export type ReportMetadata = Always<Model<"ReportMetadata">, "id" | "completed">;

export type User = Always<Model<"UserResponse">, "id" | "email" | "name" | "role_id" | "email_verified" | "created_at" | "status">;
export type Role = Always<Model<"RoleResponse">, "id" | "name">;
// The list and get always send user_count; permissions is left out when the role has none.
export type RoleDetail = Always<Model<"RoleResponse">, "id" | "name" | "description" | "user_count">;
export type PermissionInfo = Always<Model<"PermissionResponse">, "id" | "name" | "resource" | "action">;
export type RoleInput = Model<"UpdateRoleRequest">;
export type UserInput = Model<"CreateUserRequest">;
export type UserUpdate = Model<"UpdateUserRequest">;
export type ResetPasswordInput = Model<"AdminResetPasswordRequest">;
export type AuditLog = Always<Model<"AuditLogResponse">, "id" | "action" | "target_type" | "details" | "created_at">;

// The generated list response is typed for reports only, so Page<T> stays generic.
export type PageMeta = Required<Schemas["github_com_ssr0016_pastoral-reports_backend_pkg_pagination.Meta"]>;
export type Page<T> = { data: T[]; meta: PageMeta };

export type TrackerRow = Always<
  Model<"TrackerRow">,
  "user_id" | "name" | "email" | "church_name" | "user_status" | "submission_status"
>;
export type Tracker = { month: string; year: number; rows: TrackerRow[] };
