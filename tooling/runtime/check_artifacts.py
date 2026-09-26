"""Check built runtime artifacts for leaked knowledge-base, doc, or AI files.

Usage: python3 tooling/runtime/check_artifacts.py <artifact path>...

Paths may be directories (e.g. a Next.js standalone output) or files (e.g. the
Go binary). Checks:
  1. No repository-only directory names or documentation/AI files in paths.
  2. No file contains a heading/frontmatter sentinel taken from the repository's
     own Markdown knowledge base (specs, docs, skills, root instructions).
Third-party package documentation under node_modules is allowed by path rule 1
but is still scanned for repository sentinels.
"""

from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[2]
FORBIDDEN_DIRS = {"specs", "docs", ".agents", ".codex", "tooling", ".local-only", ".github"}
FORBIDDEN_NAMES = {"AGENTS.md", "CLAUDE.md"}
FORBIDDEN_SUFFIXES = {".md", ".docx"}
MAX_SCAN_BYTES = 256 * 1024 * 1024


def sentinels():
    sources = [ROOT / "AGENTS.md", ROOT / "CLAUDE.md", ROOT / "README.md"]
    for directory in ("specs", "docs", ".agents"):
        sources.extend((ROOT / directory).rglob("*.md"))
    found = set()
    for source in sources:
        if not source.is_file():
            continue
        text = source.read_text(encoding="utf-8")
        heading = re.search(r"^# .{8,}$", text, re.MULTILINE)
        if heading:
            found.add(heading.group(0).encode())
        name = re.search(r"^name: (tableflow-[a-z-]+)$", text, re.MULTILINE)
        if name:
            found.add(name.group(0).encode())
    return sorted(found)


def iter_files(path):
    if path.is_file():
        yield path
    elif path.is_dir():
        yield from (p for p in path.rglob("*") if p.is_file())


def main(argv):
    if not argv:
        print(__doc__, file=sys.stderr)
        return 2
    markers = sentinels()
    if len(markers) < 10:
        print(f"Too few sentinels extracted ({len(markers)}); check repository layout", file=sys.stderr)
        return 2
    errors, scanned = [], 0
    for arg in argv:
        base = Path(arg).resolve()
        if not base.exists():
            errors.append(f"artifact missing: {arg} (build it first)")
            continue
        for file in iter_files(base):
            rel = file.relative_to(base) if base.is_dir() else Path(file.name)
            parts = set(rel.parts[:-1])
            third_party = "node_modules" in rel.parts
            if not third_party:
                if parts & FORBIDDEN_DIRS:
                    errors.append(f"{arg}: repository-only directory in {rel}")
                if file.name in FORBIDDEN_NAMES or file.suffix.lower() in FORBIDDEN_SUFFIXES:
                    errors.append(f"{arg}: documentation/AI file {rel}")
            if file.stat().st_size > MAX_SCAN_BYTES:
                errors.append(f"{arg}: {rel} too large to scan")
                continue
            data = file.read_bytes()
            scanned += 1
            for marker in markers:
                if marker in data:
                    errors.append(f"{arg}: {rel} contains knowledge-base text {marker.decode()!r}")
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print(f"Scanned {scanned} artifact files against {len(markers)} knowledge-base sentinels: no leaks.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
