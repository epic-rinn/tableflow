"use client";

import { LayoutGrid, LogOut } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar";
import { api } from "@/lib/api/client";
import { ROLE_LABELS, type StaffIdentity } from "@/lib/api/types";
import { WORKSPACE_ICONS } from "@/lib/workspaceIcons";
import type { Workspace } from "@/lib/workspaces";

// Role-filtered workspace navigation. Hiding a link is not authorization;
// Go checks every call.
export function AppSidebar({ identity, workspaces }: { identity: StaffIdentity; workspaces: Workspace[] }) {
  const pathname = usePathname();
  const router = useRouter();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function signOut() {
    setBusy(true);
    const res = await api("/sessions/current", { method: "DELETE" });
    setBusy(false);
    if (res.ok || res.status === 401) {
      router.replace("/login");
      router.refresh();
      return;
    }
    setError(res.error.message);
  }

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <Link href="/">
                <span className="flex size-8 items-center justify-center rounded-lg bg-primary text-sm font-bold text-primary-foreground" aria-hidden>
                  TF
                </span>
                <span className="grid leading-tight">
                  <span className="font-semibold">TableFlow</span>
                  <span className="text-xs text-muted-foreground">Staff workspace</span>
                </span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Workspaces</SidebarGroupLabel>
          <SidebarGroupContent>
            <nav aria-label="Workspaces">
              <SidebarMenu>
                {workspaces.map((w) => {
                  const Icon = WORKSPACE_ICONS[w.href] ?? LayoutGrid;
                  const active = pathname === w.href || pathname.startsWith(`${w.href}/`);
                  return (
                    <SidebarMenuItem key={w.href}>
                      <SidebarMenuButton asChild isActive={active} tooltip={w.label}>
                        <Link href={w.href} aria-current={active ? "page" : undefined}>
                          <Icon aria-hidden />
                          <span>{w.label}</span>
                        </Link>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  );
                })}
              </SidebarMenu>
            </nav>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <div className="grid gap-0.5 px-2 py-1 text-sm group-data-[collapsible=icon]:hidden">
          <span className="truncate font-medium">{identity.display_name}</span>
          <span className="truncate text-xs text-muted-foreground">{identity.roles.map((r) => ROLE_LABELS[r]).join(" · ")}</span>
        </div>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton onClick={() => void signOut()} disabled={busy} tooltip="Sign out">
              <LogOut aria-hidden />
              <span>Sign out</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
        {error && (
          <p role="alert" className="px-2 text-xs text-destructive">
            {error}
          </p>
        )}
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
