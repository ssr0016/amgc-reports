import { redirect } from "next/navigation";

// The list lives in the Users tab of /admin; this keeps old links and bookmarks working.
export default function Page() {
  redirect("/admin?tab=users");
}
