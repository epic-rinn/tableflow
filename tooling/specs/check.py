from pathlib import Path
import re
import sys
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[2]
REQUIRED = (
    "AGENTS.md",
    "specs/README.md",
    "specs/product/mvp.md",
    "specs/architecture/system.md",
    "specs/architecture/repository.md",
    "specs/features/06-admin.md",
    "specs/architecture/data-model.md",
    "specs/api/http.md",
    "specs/quality/performance.md",
    "specs/quality/testing.md",
    "specs/delivery/workflow.md",
    "specs/delivery/roadmap.md",
)
SKILLS = (
    "tableflow-deliver-feature",
    "tableflow-code-review",
    "tableflow-db-api-review",
)
LINK = re.compile(r"!?\[[^\]\n]*\]\(([^)\n]+)\)")


def check_links(path):
    errors = []
    content = path.read_text(encoding="utf-8")
    outside_fences = re.sub(r"```.*?```", "", content, flags=re.DOTALL)
    for match in LINK.finditer(outside_fences):
        target = match.group(1).strip()
        if target.startswith("<"):
            target = target[1:target.index(">")]
        else:
            target = target.split(' "', 1)[0]
        parsed = urlsplit(target)
        if parsed.scheme or parsed.netloc or not parsed.path:
            continue
        resolved = (path.parent / unquote(parsed.path)).resolve()
        if not resolved.is_relative_to(ROOT):
            errors.append(f"{path.relative_to(ROOT)}: link escapes repo: {target}")
        elif not resolved.exists():
            errors.append(f"{path.relative_to(ROOT)}: missing link: {target}")
    return errors


def check_skill(name):
    errors = []
    folder = ROOT / ".agents" / "skills" / name
    entry = folder / "SKILL.md"
    if not entry.is_file():
        return [f"Missing skill: {entry.relative_to(ROOT)}"]
    content = entry.read_text(encoding="utf-8")
    frontmatter = re.match(r"\A---\n(.*?)\n---\n", content, re.DOTALL)
    if not frontmatter:
        return [f"Invalid skill frontmatter: {name}"]
    fields = dict(re.findall(r"^([a-z_-]+):\s*(.+)$", frontmatter.group(1), re.MULTILINE))
    if fields.get("name") != name or not fields.get("description"):
        errors.append(f"Skill requires matching name and nonempty description: {name}")
    metadata = folder / "agents" / "openai.yaml"
    if not metadata.is_file():
        errors.append(f"Missing UI metadata: {name}")
    elif f"${name}" not in metadata.read_text(encoding="utf-8"):
        errors.append(f"Missing skill invocation in UI prompt: {name}")
    return errors


def check_runtime_boundaries():
    errors = []
    for project in ("admin", "pwa", "api"):
        if not (ROOT / "src" / project).is_dir():
            errors.append(f"Missing runtime project: src/{project}")
    for path in (ROOT / "src").rglob("*"):
        relative = path.relative_to(ROOT / "src")
        if {"node_modules", ".next", "vendor", ".git"}.intersection(relative.parts):
            continue
        if path.is_file() and path.suffix.lower() == ".md":
            errors.append(f"Move documentation/AI instructions outside src: {path.relative_to(ROOT)}")
        if path.is_dir() and path.name in {".agents", ".codex", "specs", "docs"}:
            errors.append(f"Non-runtime directory inside src: {path.relative_to(ROOT)}")
    return errors


def main():
    errors = [f"Missing required file: {name}" for name in REQUIRED if not (ROOT / name).is_file()]
    documents = list(ROOT.glob("*.md"))
    for directory in ("specs", ".agents/skills", "docs"):
        documents.extend(
            document
            for document in (ROOT / directory).rglob("*.md")
            if not {"node_modules", ".next", "vendor", "__pycache__"}.intersection(document.relative_to(ROOT).parts)
        )
    for document in documents:
        errors.extend(check_links(document))
    for skill in SKILLS:
        errors.extend(check_skill(skill))
    errors.extend(check_runtime_boundaries())
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    print(f"Checked {len(documents)} Markdown files and {len(SKILLS)} skills.")
    print("Structure, source boundaries, and local file links passed; runtime behavior, anchors, external URLs, and review quality are not checked.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
