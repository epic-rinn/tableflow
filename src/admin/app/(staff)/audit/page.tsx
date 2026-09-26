import { Forbidden } from "@/components/Forbidden";
import { AuditLog } from "@/components/reports/AuditLog";
import { getSession } from "@/lib/api/server";

export default async function AuditPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  if (!session.identity.roles.includes("manager")) return <Forbidden />;
  return <AuditLog branchId={session.identity.branch_id} />;
}
