import { CashierWorkspace } from "@/components/cashier/CashierWorkspace";
import { Forbidden } from "@/components/Forbidden";
import { getSession } from "@/lib/api/server";

export default async function CashierPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  const { roles, branch_id } = session.identity;
  if (!roles.includes("cashier") && !roles.includes("manager")) return <Forbidden />;
  return <CashierWorkspace branchId={branch_id} />;
}
