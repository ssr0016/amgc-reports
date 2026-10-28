import { Suspense } from "react";
import { FormSkeleton } from "@/components/loading-skeletons";
import { EditRole } from "./edit-role";

export const metadata = { title: "Edit role" };

// A Client Component page cannot export `instant`, so this Server Component
// page does. The screen loads after navigation by design (it needs the session).
export const instant = false;

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <Suspense fallback={<FormSkeleton fields={3} />}><EditRole id={Number(id)} /></Suspense>;
}
