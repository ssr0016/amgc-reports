import { Button } from "@/components/ui/button";

// A failed fetch: the message, and a Retry that runs the same query again.
export function ErrorRetry({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="flex flex-wrap items-center gap-4">
      <p role="alert" className="text-sm text-destructive">
        {message}
      </p>
      <Button variant="outline" size="sm" onClick={onRetry}>
        Retry
      </Button>
    </div>
  );
}
