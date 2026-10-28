import type { components } from "./api-types";

type Schemas = components["schemas"];
type Models = {
  [K in keyof Schemas as K extends `github_com_ssr0016_pastoral-reports_backend_internal_model.${infer N}`
    ? N
    : never]: Schemas[K];
};
type Model<N extends keyof Models> = Models[N];
type StatsModels = {
  [K in keyof Schemas as K extends `github_com_ssr0016_pastoral-reports_backend_internal_stats.${infer N}` ? N : never]: Schemas[K];
};
type StatsModel<N extends keyof StatsModels> = StatsModels[N];

// swag marks a response field required only when the Go field has no omitempty
// and a validate tag, so the generated types make everything optional. The
// server always sends the fields listed here, so they are made required.
type Always<T, K extends keyof T> = Required<Pick<T, K>> & Omit<T, K>;

export type Me = Required<Model<"MeResponse">>;
export type RoleCount = Required<StatsModel<"RoleCount">>;
export type StatusCount = Required<StatsModel<"StatusCount">>;
// The generated items are all-optional; the server always fills them.
export type AdminStats = Omit<Required<Model<"AdminStats">>, "role_counts" | "status_counts"> & {
  role_counts: RoleCount[];
  status_counts: StatusCount[];
};

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

// The analytics answer (D-A31). Every field is always sent except the ones marked
// optional (omitempty in Go); a null number is "no data", never 0.
type N = number | null;
export type AnalyticsMonth = { month: number; name: string; state: "not_yet_open" | "open" | "grace" | "closed" };
export type ChurchOption = { id: number; name: string };
export type ChurchScope = ChurchOption & {
  start_month?: string; // effective: the church's override, else the organization default
  start_is_default: boolean;
  end_month?: string;
  needs_review: boolean;
  active: boolean; // has an active church account, as the admin cards count churches
  account_status: string;
  workers: string;
};
// expected_reported: reports in scope for owed church-months; outside_reports: for months not required.
export type Coverage = { months: number; reports_in_scope: number; expected_reported: number; outside_reports: number; churches_reported: number; expected: number; churches: number };
export type ChurchValue = { church_id: number; value: N; observations: number; state: "value" | "blank" | "not_approved" | "no_report" };
export type MetricSummary = {
  metric: string;
  label: string;
  group: string;
  unit: "people" | "php" | "count" | "as_reported";
  total: N;
  observations: number;
  zero_observations: number;
  reports_with_value: number;
  churches_with_value: number;
  avg_per_church_service?: N;
  network_weekly?: N;
  network_slots: number;
  network_slots_full: number;
  peak_month: string;
  peak_value: number;
  churches: ChurchValue[];
};
export type TrendPoint = {
  month: number;
  name: string;
  state: AnalyticsMonth["state"];
  counted: boolean;
  value: N;
  observations: number;
  churches_with_value: number;
  churches_reported: number;
  expected_reported: number; // reported among the churches that owed the month
  churches_expected: number;
  partial: boolean;
};
export type SlotPoint = { month: number; week: number; total: N; churches_with_value: number; churches_reported: number };
export type ChurchMetric = {
  church_id: number;
  church_name: string;
  total: N;
  observations: number;
  average?: N;
  previous_total?: N;
  previous_observations: number;
  previous_average?: N;
  change?: N;
  tithes_total: N;
  submitted: number;
  approved: number;
  expected: number;
  completeness: N;
};
export type ComplianceCounts = {
  expected: number;
  outside: number; // reports for months the church did not owe: kept, not in expected
  submitted: number;
  approved: number;
  pending: number;
  needs_revision: number;
  on_time: number;
  late: number;
  awaiting: number;
  overdue: number;
  not_filed: number;
  duplicates: number;
};
export type ComplianceCell = {
  month: number;
  expected: boolean;
  state: "not_required" | "not_reviewed" | "inactive" | "not_yet_open" | "awaiting_submission" | "overdue_grace" | "not_filed" | "submitted";
  report_id?: number;
  review_status?: "approved" | "pending" | "needs_revision";
  timeliness?: "on_time" | "late" | "after_cutoff" | "before_open";
  submitted_on?: string;
  duplicates?: number;
};
export type ComplianceRow = ComplianceCounts & { church_id: number; church_name: string; cells: ComplianceCell[] };
export type LikeForLike = {
  churches: number;
  current_total: number;
  current_observations: number;
  previous_total: number;
  previous_observations: number;
  change?: N;
};
export type AttendanceComparison = {
  year: number;
  coverage: Coverage;
  metrics: MetricSummary[];
  trend: TrendPoint[];
  slots: SlotPoint[];
  like_for_like: LikeForLike;
};
export type AttendanceAnalytics = {
  year: number;
  metric: string;
  status: string;
  period: string;
  generated_at: string;
  default_start_month: string; // "YYYY-MM", "" when not configured
  months: AnalyticsMonth[];
  cutoff_month: number;
  church_options: ChurchOption[];
  churches: ChurchScope[];
  coverage: Coverage;
  metrics: MetricSummary[];
  trend: TrendPoint[];
  slots: SlotPoint[];
  church_rows: ChurchMetric[];
  compliance: ComplianceRow[];
  compliance_totals: ComplianceCounts & { churches_with_submission: number; churches_needing_review: number };
  comparison?: AttendanceComparison;
};
export type ChurchReportingInput = Required<StatsModel<"ChurchReportingRequest">>;
