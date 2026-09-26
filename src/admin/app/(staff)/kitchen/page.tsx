import { Forbidden } from "@/components/Forbidden";
import { KitchenBoard } from "@/components/kitchen/KitchenBoard";
import { getSession } from "@/lib/api/server";

export default async function KitchenPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  const { roles, branch_id } = session.identity;
  if (!roles.includes("kitchen") && !roles.includes("manager")) return <Forbidden />;
  return <KitchenBoard branchId={branch_id} isManager={roles.includes("manager")} />;
}
