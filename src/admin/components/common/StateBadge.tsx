import { cn } from "@/lib/utils";

type Tone = "success" | "warning" | "info" | "destructive" | "muted" | "primary";

const TONES: Record<Tone, string> = {
  success: "bg-success-soft text-success",
  warning: "bg-warning-soft text-warning",
  info: "bg-info-soft text-info",
  destructive: "bg-destructive-soft text-destructive",
  muted: "bg-muted text-muted-foreground",
  primary: "bg-accent text-accent-foreground",
};

// One mapping for every operational state (UI design: text + colour).
const STATE_TONES: Record<string, Tone> = {
  available: "success",
  held: "warning",
  occupied: "info",
  cleaning: "muted",
  waiting: "info",
  called: "warning",
  seated: "success",
  cancelled: "destructive",
  no_show: "destructive",
  submitted: "info",
  accepted: "primary",
  preparing: "warning",
  ready: "success",
  served: "muted",
  rejected: "destructive",
  open: "info",
  settling: "warning",
  paid: "success",
  departed: "muted",
  closed: "muted",
  acknowledged: "primary",
  resolved: "muted",
  refunded: "destructive",
  active: "success",
  invited: "info",
  deactivated: "muted",
};

export function StateBadge({ state, label, tone, className }: { state?: string; label?: React.ReactNode; tone?: Tone; className?: string }) {
  const t = tone ?? (state ? STATE_TONES[state] : undefined) ?? "muted";
  return (
    <span className={cn("inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap", TONES[t], className)}>
      <span className="size-1.5 rounded-full bg-current" aria-hidden />
      {label ?? state?.replace("_", " ")}
    </span>
  );
}
