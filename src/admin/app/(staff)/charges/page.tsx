import { ChargePolicyEditor } from "@/components/ChargePolicyEditor";
import { Forbidden } from "@/components/Forbidden";
import { getSession } from "@/lib/api/server";

export default async function ChargesPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  if (!session.identity.roles.includes("manager")) return <Forbidden />;
  return <ChargePolicyEditor branchId={session.identity.branch_id} />;
}
