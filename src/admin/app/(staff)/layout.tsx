import Link from "next/link";
import { redirect } from "next/navigation";
import { SignOutButton } from "@/components/SignOutButton";
import { getSession } from "@/lib/api/server";
import { workspacesFor } from "@/lib/workspaces";

export default async function StaffLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const session = await getSession();
  if (session.state === "anonymous") redirect("/login");
  if (session.state === "unavailable") {
    return (
      <main>
        <h1>Service unavailable</h1>
        <p role="alert">TableFlow cannot reach its server right now. Reload the page to try again.</p>
      </main>
    );
  }
  const { identity } = session;
  return (
    <>
      <header className="bar">
        <Link href="/">TableFlow Admin</Link>
        <nav aria-label="Workspaces">
          <ul>
            {workspacesFor(identity).map((w) => (
              <li key={w.href}>
                <Link href={w.href}>{w.label}</Link>
              </li>
            ))}
          </ul>
        </nav>
        <span>
          {identity.display_name} <SignOutButton />
        </span>
      </header>
      <main>{children}</main>
    </>
  );
}
