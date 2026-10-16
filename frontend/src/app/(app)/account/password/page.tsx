import { ChangePassword } from "./change-password";

export const metadata = { title: "Change password" };

// A Client Component page cannot export `instant`, so this Server Component
// page does. The form loads after navigation by design (it needs the session).
export const instant = false;

export default function Page() {
  return <ChangePassword />;
}
