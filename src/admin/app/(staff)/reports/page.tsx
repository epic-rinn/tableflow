import { Forbidden } from "@/components/Forbidden";
import { DailyReports } from "@/components/reports/DailyReports";
import { getSession } from "@/lib/api/server";

export default async function ReportsPage() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  if (!session.identity.roles.includes("manager")) return <Forbidden />;
  return <DailyReports branchId={session.identity.branch_id} />;
}
