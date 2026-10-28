#!/usr/bin/env python3
"""Check local file targets in repository documentation (no external requests)."""

from pathlib import Path
import re
import sys
from urllib.parse import unquote, urlsplit


root = Path(__file__).resolve().parent.parent
files = sorted({
    root / "README.md",
    root / "CHANGELOG.md",
    root / "frontend/README.md",
    *root.glob("docs/**/*.md"),
    *root.glob("deploy/**/*.md"),
})
count = 0
errors = []
for file in files:
    # Current docs use inline Markdown links; anchors and remote URLs are excluded.
    for number, line in enumerate(file.read_text(encoding="utf-8").splitlines(), 1):
        for match in re.finditer(r"!?\[[^\]]*\]\(([^)]+)\)", line):
            target = match.group(1).split(' "', 1)[0].strip("<>")
            url = urlsplit(target)
            if url.scheme or url.netloc or not url.path:
                continue
            count += 1
            destination = (file.parent / unquote(url.path)).resolve()
            if not destination.is_relative_to(root) or not destination.exists():
                errors.append(f"{file.relative_to(root)}:{number}: {target}")

if errors:
    print("Broken or nonportable documentation links:\n" + "\n".join(errors), file=sys.stderr)
    sys.exit(1)
print(f"PASS: {count} local file links in {len(files)} Markdown documents")
