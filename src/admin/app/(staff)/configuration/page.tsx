import { Forbidden } from "@/components/Forbidden";
import { FloorConfig } from "@/components/FloorConfig";
import { getSession } from "@/lib/api/server";

export default async function ConfigurationPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  if (!session.identity.roles.includes("manager")) return <Forbidden />;
  return <FloorConfig branchId={session.identity.branch_id} />;
}
