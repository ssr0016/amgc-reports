import { Suspense } from "react";
import { TableSkeleton } from "@/components/loading-skeletons";
import { EditReport } from "./edit-report";

export const metadata = { title: "Edit report" };

// A Client Component page cannot export `instant`, so this Server Component
// page does. The form loads after navigation by design (it needs the session).
export const instant = false;

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <Suspense fallback={<TableSkeleton columns={2} rows={4} />}><EditReport id={Number(id)} /></Suspense>;
}
