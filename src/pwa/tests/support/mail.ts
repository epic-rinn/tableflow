import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";

// Reads links from the test-only file outbox (MAIL_ADAPTER=file) that
// `make verify` points at E2E_MAIL_DIR.
const MAIL_DIR = process.env.E2E_MAIL_DIR ?? "";

export async function latestLink(to: string, path: string): Promise<string> {
  const pattern = new RegExp(`(${path.replace(/\//g, "\\/")}#[A-Za-z0-9_-]{43})`);
  for (let i = 0; i < 50; i++) {
    const files = (await readdir(MAIL_DIR)).sort().reverse();
    for (const f of files) {
      const m = JSON.parse(await readFile(join(MAIL_DIR, f), "utf8")) as { To: string; Text: string };
      const match = m.To === to ? m.Text.match(pattern) : null;
      if (match) return match[1];
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`no ${path} email for ${to}`);
}
