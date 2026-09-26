import { ReceiptList } from "@/components/cashier/Receipts";
import { Forbidden } from "@/components/Forbidden";
import { getSession } from "@/lib/api/server";
import { isDate } from "@/lib/dates";

export default async function ReceiptsPage({ searchParams }: { searchParams: Promise<{ from?: string; to?: string }> }) {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  const { roles, branch_id } = session.identity;
  if (!roles.includes("cashier") && !roles.includes("manager")) return <Forbidden />;
  const { from, to } = await searchParams;
  return <ReceiptList branchId={branch_id} from={isDate(from) ? from : undefined} to={isDate(to) ? to : undefined} />;
}
