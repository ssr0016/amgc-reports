import { NewUser } from "./new-user";

export const metadata = { title: "New user" };

// A Client Component page cannot export `instant`, so this Server Component
// page does. The screen loads after navigation by design (it needs the session).
export const instant = false;

export default function Page() {
  return <NewUser />;
}
