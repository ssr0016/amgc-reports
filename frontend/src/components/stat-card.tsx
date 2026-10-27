import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

// Icon tiles, dots and glows are decorative; the label carries the meaning.
const TONE = {
  neutral: { tile: "bg-muted text-foreground", dot: "bg-muted-foreground", glow: "hover:ring-white/15 hover:shadow-white/5" },
  green: { tile: "bg-primary/10 text-primary", dot: "bg-primary", glow: "hover:ring-primary/30 hover:shadow-primary/15" },
  sky: { tile: "bg-sky-500/10 text-sky-400", dot: "bg-sky-400", glow: "hover:ring-sky-400/30 hover:shadow-sky-500/15" },
  amber: { tile: "bg-amber-500/10 text-amber-400", dot: "bg-amber-400", glow: "hover:ring-amber-400/30 hover:shadow-amber-500/15" },
  red: { tile: "bg-red-500/10 text-red-400", dot: "bg-red-400", glow: "hover:ring-red-400/30 hover:shadow-red-500/15" },
};

export type StatTone = keyof typeof TONE;

// value is optional so a breakdown card can carry its rows in note alone.
export function StatCard({ label, value, icon: Icon, tone, note }: { label: string; value?: ReactNode; icon: LucideIcon; tone: StatTone; note?: ReactNode }) {
  return (
    <Card
      className={`group border-white/[0.06] bg-card/80 bg-linear-to-b from-white/[0.03] to-transparent shadow-sm ring-1 ring-transparent transition-shadow duration-200 hover:shadow-lg ${TONE[tone].glow}`}
    >
      <CardHeader className="gap-2">
        <CardDescription className="text-xs font-medium tracking-wide uppercase">{label}</CardDescription>
        {value !== undefined && (
          <CardTitle className="flex items-center gap-2 text-3xl font-semibold tracking-tight tabular-nums">
            <span aria-hidden="true" className={`size-1.5 rounded-full ${TONE[tone].dot}`} />
            {value}
          </CardTitle>
        )}
        <CardAction>
          <span aria-hidden="true" className={`grid size-11 place-items-center rounded-lg transition-transform duration-200 motion-safe:group-hover:scale-110 ${TONE[tone].tile}`}>
            <Icon className="size-6" />
          </span>
        </CardAction>
      </CardHeader>
      {note && <CardContent className="text-muted-foreground">{note}</CardContent>}
    </Card>
  );
}
