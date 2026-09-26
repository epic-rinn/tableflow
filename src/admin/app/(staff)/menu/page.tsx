import { Forbidden } from "@/components/Forbidden";
import { MenuEditor } from "@/components/MenuEditor";
import { getSession } from "@/lib/api/server";

export default async function MenuPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  if (!session.identity.roles.includes("manager")) return <Forbidden />;
  return <MenuEditor branchId={session.identity.branch_id} />;
}
