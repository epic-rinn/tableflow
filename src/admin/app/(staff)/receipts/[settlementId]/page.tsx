import { ReceiptDetail } from "@/components/cashier/Receipts";
import { Forbidden } from "@/components/Forbidden";
import { getSession } from "@/lib/api/server";

export default async function ReceiptPage({ params }: { params: Promise<{ settlementId: string }> }) {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  const { roles } = session.identity;
  if (!roles.includes("cashier") && !roles.includes("manager")) return <Forbidden />;
  const { settlementId } = await params;
  return <ReceiptDetail id={settlementId} isManager={roles.includes("manager")} />;
}
