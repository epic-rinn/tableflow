import { CircleAlert, CircleCheck } from "lucide-react";
import { cn } from "@/lib/utils";

export type NoticeValue = { role: "alert" | "status"; text: string } | null;

// Inline result message; role alert/status is what screen readers and the
// browser tests rely on (toasts are never the only feedback).
export function Notice({ notice, className }: { notice: NoticeValue; className?: string }) {
  if (!notice) return null;
  const error = notice.role === "alert";
  const Icon = error ? CircleAlert : CircleCheck;
  return (
    <p
      role={notice.role}
      className={cn(
        "flex items-start gap-2 rounded-lg border px-3 py-2 text-sm",
        error ? "border-destructive/30 bg-destructive-soft text-destructive" : "border-success/30 bg-success-soft text-success",
        className,
      )}
    >
      <Icon className="mt-0.5 size-4 shrink-0" aria-hidden />
      <span>{notice.text}</span>
    </p>
  );
}
