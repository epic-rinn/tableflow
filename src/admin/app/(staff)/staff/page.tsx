import { Forbidden } from "@/components/Forbidden";
import { StaffManager } from "@/components/StaffManager";
import { getSession } from "@/lib/api/server";

export default async function StaffPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  if (!session.identity.roles.includes("manager")) return <Forbidden />;
  return <StaffManager branchId={session.identity.branch_id} selfId={session.identity.staff_id} />;
}
