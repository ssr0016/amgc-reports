import { api } from "./api";
import type { components } from "./api-types";

type NotificationSchema = components["schemas"]["github_com_ssr0016_pastoral-reports_backend_internal_notification.NotificationResponse"];

export type Notification = Required<Pick<NotificationSchema, "id" | "type" | "message" | "metadata" | "created_at">> &
  Pick<NotificationSchema, "read_at">;

export const listUnreadNotifications = () => api<Notification[]>("/notifications");

export const markNotificationRead = (id: number) =>
  api<void>(`/notifications/${id}/read`, { method: "PATCH" });
