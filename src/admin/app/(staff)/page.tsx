import { ArrowRight, LayoutGrid } from "lucide-react";
import Link from "next/link";
import { PageHeader } from "@/components/common/PageHeader";
import { WORKSPACE_ICONS } from "@/lib/workspaceIcons";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { getSession } from "@/lib/api/server";
import { ROLE_LABELS } from "@/lib/api/types";
import { workspacesFor } from "@/lib/workspaces";

const DESCRIPTIONS: Record<string, string> = {
  "/host": "Queue, table board, seating and assistance",
  "/kitchen": "Active order lines and sold-out items",
  "/cashier": "Bills, settlement and payment confirmation",
  "/receipts": "Receipt history and refunds",
  "/configuration": "Tables and seating groups",
  "/menu": "Categories, items, options and prices",
  "/reports": "Daily sales, refunds, queue and loyalty totals",
  "/audit": "Who changed what, when and why",
  "/charges": "Service charge and tax policy",
  "/loyalty": "Points rate, tiers and member discounts",
  "/staff": "Invitations, roles and access",
};

export default async function Home() {
  const session = await getSession();
  if (session.state !== "authenticated") return null; // layout handles other states
  const { identity } = session;
  return (
    <>
      <PageHeader title={`Welcome, ${identity.display_name}`} description={`Roles: ${identity.roles.map((r) => ROLE_LABELS[r]).join(", ")}`} />
      <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {workspacesFor(identity).map((w) => {
          const Icon = WORKSPACE_ICONS[w.href] ?? LayoutGrid;
          return (
            <li key={w.href}>
              <Link href={w.href} className="group block rounded-xl focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none">
                <Card className="h-full transition-colors group-hover:border-primary/40 group-hover:bg-accent/40">
                  <CardHeader>
                    <div className="mb-2 flex size-10 items-center justify-center rounded-lg bg-accent text-accent-foreground">
                      <Icon className="size-5" aria-hidden />
                    </div>
                    <CardTitle className="flex items-center justify-between">
                      {w.label}
                      <ArrowRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" aria-hidden />
                    </CardTitle>
                    <CardDescription>{DESCRIPTIONS[w.href]}</CardDescription>
                  </CardHeader>
                </Card>
              </Link>
            </li>
          );
        })}
      </ul>
    </>
  );
}
