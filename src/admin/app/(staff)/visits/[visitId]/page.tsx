import { Forbidden } from "@/components/Forbidden";
import { VisitOrders } from "@/components/VisitOrders";
import { getSession } from "@/lib/api/server";

export default async function VisitPage({ params }: { params: Promise<{ visitId: string }> }) {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  const { roles, branch_id } = session.identity;
  if (!roles.includes("host") && !roles.includes("manager")) return <Forbidden />;
  const { visitId } = await params;
  return <VisitOrders branchId={branch_id} visitId={visitId} isManager={roles.includes("manager")} />;
}
