import { ReceiptList } from "@/components/cashier/Receipts";
import { Forbidden } from "@/components/Forbidden";
import { getSession } from "@/lib/api/server";

export default async function ReceiptsPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  const { roles, branch_id } = session.identity;
  if (!roles.includes("cashier") && !roles.includes("manager")) return <Forbidden />;
  return <ReceiptList branchId={branch_id} />;
}
