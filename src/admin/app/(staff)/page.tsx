import Link from "next/link";
import { getSession } from "@/lib/api/server";
import { ROLE_LABELS } from "@/lib/api/types";
import { workspacesFor } from "@/lib/workspaces";

export default async function Home() {
  const session = await getSession();
  if (session.state !== "authenticated") return null; // layout handles other states
  const { identity } = session;
  return (
    <>
      <h1>Welcome, {identity.display_name}</h1>
      <p>Roles: {identity.roles.map((r) => ROLE_LABELS[r]).join(", ")}</p>
      <ul>
        {workspacesFor(identity).map((w) => (
          <li key={w.href}>
            <Link href={w.href}>{w.label}</Link>
          </li>
        ))}
      </ul>
    </>
  );
}
