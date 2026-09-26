import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

// Styled native <select>: keyboard/mobile friendly and scriptable in tests.
export function NativeSelect({ className, ...props }: React.ComponentProps<"select">) {
  return (
    <span className={cn("relative inline-flex", className)}>
      <select
        className="h-8 w-full appearance-none rounded-lg border border-input bg-background py-1 pr-8 pl-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"
        {...props}
      />
      <ChevronDown className="pointer-events-none absolute top-1/2 right-2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
    </span>
  );
}
