import { Suspense } from "react";
import { TableSkeleton } from "@/components/loading-skeletons";
import { EditReport } from "./edit/edit-report";

export const metadata = { title: "View report" };

export const instant = false;

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <Suspense fallback={<TableSkeleton columns={2} rows={4} />}><EditReport id={Number(id)} viewOnly /></Suspense>;
}
