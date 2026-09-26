import { Forbidden } from "@/components/Forbidden";
import { getSession } from "@/lib/api/server";

export default async function Workspace() {
  const session = await getSession();
  if (session.state !== "authenticated") return null;
  if (!session.identity.roles.includes("cashier")) return <Forbidden />;
  return (
    <>
      <h1>Cashier</h1>
      <p>This workspace is not available yet.</p>
    </>
  );
}
