import { ReportDetail } from "./report-detail";

// A Client Component page cannot export `instant`, so this Server Component
// page does. The report loads after navigation by design (it needs the session).
export const instant = false;

export default function Page() {
  return <ReportDetail />;
}
