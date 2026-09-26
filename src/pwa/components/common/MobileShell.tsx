import { LanguageSwitch } from "@/components/common/LanguageSwitch";
import { cn } from "@/lib/utils";

// Phone-first page frame: coloured hero header, then content on a sheet-like
// surface. Capped at ~480 px on larger screens (UI design, PWA layout).
export function MobileShell({ eyebrow = "TableFlow", title, hero, children, className }: {
  eyebrow?: string; title: React.ReactNode; hero?: React.ReactNode; children: React.ReactNode; className?: string;
}) {
  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-md flex-col bg-background shadow-sm">
      <header className="bg-primary px-5 pt-[max(1.25rem,env(safe-area-inset-top))] pb-10 text-primary-foreground">
        <div className="flex items-center justify-between gap-3">
          <p className="text-xs font-semibold tracking-wide uppercase">{eyebrow}</p>
          <LanguageSwitch />
        </div>
        <h1 className="mt-1 text-2xl font-bold">{title}</h1>
        {hero}
      </header>
      <div className={cn("-mt-6 flex flex-1 flex-col gap-4 rounded-t-3xl bg-background px-4 pt-5 pb-safe", className)}>{children}</div>
    </main>
  );
}

// Rounded status chip used for queue, visit and line states.
export function Pill({ tone = "muted", children }: { tone?: "success" | "warning" | "info" | "destructive" | "muted" | "primary"; children: React.ReactNode }) {
  const tones = {
    success: "bg-success-soft text-success",
    warning: "bg-warning-soft text-warning",
    info: "bg-info-soft text-info",
    destructive: "bg-destructive-soft text-destructive",
    muted: "bg-muted text-muted-foreground",
    primary: "bg-accent text-accent-foreground",
  };
  return <span className={cn("inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold whitespace-nowrap", tones[tone])}>{children}</span>;
}
