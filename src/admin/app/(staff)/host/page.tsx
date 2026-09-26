import { Forbidden } from "@/components/Forbidden";
import { HostWorkspace } from "@/components/host/HostWorkspace";
import { getSession } from "@/lib/api/server";

export default async function HostPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  const { roles, branch_id } = session.identity;
  if (!roles.includes("host") && !roles.includes("manager")) return <Forbidden />;
  return <HostWorkspace branchId={branch_id} isManager={roles.includes("manager")} />;
}
