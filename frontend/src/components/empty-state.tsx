import Link from "next/link";
import { FileText, type LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";

export function EmptyState({
  title,
  description,
  action,
  icon: Icon = FileText,
}: {
  title: string;
  description: string;
  action?: { href: string; label: string };
  icon?: LucideIcon;
}) {
  return (
    <div role="status" className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed p-8 text-center">
      <div className="flex size-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
        <Icon aria-hidden="true" className="size-6" />
      </div>
      <div className="flex flex-col gap-1">
        <p className="font-medium">{title}</p>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>
      {action && <Button asChild><Link href={action.href}>{action.label}</Link></Button>}
    </div>
  );
}
