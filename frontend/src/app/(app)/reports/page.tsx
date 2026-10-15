import { Reports } from "./reports-list";

export const metadata = { title: "Reports" };

// A Client Component page cannot export `instant`, so this Server Component
// page does. The list loads after navigation by design (it needs the session).
export const instant = false;

export default function Page() {
  return <Reports />;
}
