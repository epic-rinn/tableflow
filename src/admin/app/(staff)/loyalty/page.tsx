import { Forbidden } from "@/components/Forbidden";
import { LoyaltyPolicyEditor } from "@/components/LoyaltyPolicyEditor";
import { getSession } from "@/lib/api/server";

export default async function LoyaltyPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  if (!session.identity.roles.includes("manager")) return <Forbidden />;
  return <LoyaltyPolicyEditor branchId={session.identity.branch_id} />;
}
