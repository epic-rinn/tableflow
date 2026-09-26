import { Card, CardContent, CardDescription, CardHeader } from "@/components/ui/card";

// Centred card with a brand panel for sign-in and activation screens.
export function AuthShell({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return (
    <main className="grid min-h-dvh lg:grid-cols-2">
      <div className="hidden flex-col justify-between bg-primary p-10 text-primary-foreground lg:flex">
        <div className="flex items-center gap-2 text-lg font-semibold">
          <span className="flex size-9 items-center justify-center rounded-lg border border-primary-foreground/60 font-bold" aria-hidden>
            TF
          </span>
          TableFlow
        </div>
        <div className="grid gap-2">
          <p className="text-3xl font-semibold leading-tight">Queue, tables, kitchen and cashier in one calm workspace.</p>
          <p className="text-primary-foreground">Staff workspace for your restaurant.</p>
        </div>
      </div>
      <div className="flex items-center justify-center p-6">
        <Card className="w-full max-w-sm">
          <CardHeader>
            <h1 className="text-xl font-semibold">{title}</h1>
            {description && <CardDescription>{description}</CardDescription>}
          </CardHeader>
          <CardContent>{children}</CardContent>
        </Card>
      </div>
    </main>
  );
}

export function FieldError({ id, message }: { id: string; message?: string }) {
  if (!message) return null;
  return (
    <p id={id} className="text-xs text-destructive">
      {message}
    </p>
  );
}
