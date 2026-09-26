import { redirect } from "next/navigation";
import { AppSidebar } from "@/components/shell/AppSidebar";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Separator } from "@/components/ui/separator";
import { SidebarInset, SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { getSession } from "@/lib/api/server";
import { workspacesFor } from "@/lib/workspaces";

export default async function StaffLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const session = await getSession();
  if (session.state === "anonymous") redirect("/login");
  if (session.state === "unavailable") {
    return (
      <main className="mx-auto max-w-lg p-6">
        <h1 className="mb-4 text-2xl font-semibold">Service unavailable</h1>
        <Alert variant="destructive">
          <AlertTitle>Cannot reach the server</AlertTitle>
          <AlertDescription role="alert">TableFlow cannot reach its server right now. Reload the page to try again.</AlertDescription>
        </Alert>
      </main>
    );
  }
  const { identity } = session;
  return (
    <SidebarProvider>
      <AppSidebar identity={identity} workspaces={workspacesFor(identity)} />
      <SidebarInset>
        <header className="sticky top-0 z-10 flex h-14 items-center gap-2 border-b bg-background/90 px-4 backdrop-blur">
          <SidebarTrigger aria-label="Toggle navigation" />
          <Separator orientation="vertical" className="mr-1 h-5" />
          <span className="text-sm text-muted-foreground">Signed in as {identity.display_name}</span>
          <div className="ml-auto">
            <ThemeToggle />
          </div>
        </header>
        <div className="flex-1 p-4 md:p-6">{children}</div>
      </SidebarInset>
    </SidebarProvider>
  );
}
