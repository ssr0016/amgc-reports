import { api } from "./api";
import type { AdminStats } from "./api-schema";

export type { AdminStats };

// Counts for the previous month (Manila time); the endpoint also takes ?month=&year=, which no screen uses yet.
export const getStats = () => api<AdminStats>("/admin/stats");
