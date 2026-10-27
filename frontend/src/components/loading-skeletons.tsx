import { Skeleton } from "@/components/ui/skeleton";

export function TableSkeleton({ columns = 4, rows = 6 }: { columns?: number; rows?: number }) {
  return (
    <div role="status" aria-label="Loading table" className="overflow-hidden rounded-lg border">
      <span className="sr-only">Loading table</span>
      <div className="grid gap-4 border-b bg-muted/30 p-4" style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}>
        {Array.from({ length: columns }, (_, index) => <Skeleton key={index} className="h-3 w-20" />)}
      </div>
      <div className="divide-y">
        {Array.from({ length: rows }, (_, row) => (
          <div key={row} className="grid gap-4 p-4" style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}>
            {Array.from({ length: columns }, (_, column) => <Skeleton key={column} className={`h-4 ${column === 0 ? "w-32" : "w-20"}`} />)}
          </div>
        ))}
      </div>
    </div>
  );
}

export function KpiSkeleton({ count = 4 }: { count?: number }) {
  return (
    <div role="status" aria-label="Loading summary" className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      <span className="sr-only">Loading summary</span>
      {Array.from({ length: count }, (_, index) => (
        <div key={index} className="rounded-lg border p-4">
          <Skeleton className="mb-3 h-3 w-24" />
          <Skeleton className="h-8 w-16" />
        </div>
      ))}
    </div>
  );
}

export function AnalyticsSkeleton() {
  return (
    <div className="flex flex-col gap-6" role="status" aria-label="Loading analytics">
      <span className="sr-only">Loading analytics</span>
      <KpiSkeleton count={3} />
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="h-72 rounded-lg border p-6"><Skeleton className="mb-6 h-5 w-32" /><Skeleton className="h-48 w-full" /></div>
        <div className="h-72 rounded-lg border p-6"><Skeleton className="mb-6 h-5 w-32" /><Skeleton className="h-48 w-full" /></div>
      </div>
      <TableSkeleton columns={4} rows={5} />
    </div>
  );
}

export function ProfileSkeleton() {
  return (
    <div role="status" aria-label="Loading profile" className="flex flex-col gap-6">
      <span className="sr-only">Loading profile</span>
      <div className="rounded-lg border p-6"><Skeleton className="mb-6 h-5 w-32" /><div className="grid gap-4 sm:grid-cols-2">{Array.from({ length: 5 }, (_, index) => <Skeleton key={index} className="h-11 w-full" />)}</div></div>
      <div className="rounded-lg border p-6"><div className="flex items-center gap-4"><Skeleton className="size-20 rounded-full" /><div><Skeleton className="mb-3 h-5 w-32" /><Skeleton className="h-4 w-56" /></div></div></div>
    </div>
  );
}
