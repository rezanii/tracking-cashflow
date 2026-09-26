#!/usr/bin/env bash
# Publish the backend to a Hugging Face Docker Space.
#
# A Space is its own git repository, so the code has to be pushed there. Rather than keeping a
# second copy of the backend in this repo, this assembles the Space contents from
# apps/backend plus the Space README, leaving one source of truth.
#
# Usage:
#   deploy/huggingface/sync.sh <space-git-url>
#
# The URL comes from the Space page, e.g.
#   https://huggingface.co/spaces/<user>/tracking-cashflow-api
#
# Authentication: `hf auth login` beforehand, or put a write token in HF_TOKEN.
set -euo pipefail

SPACE_URL="${1:-${HF_SPACE_URL:-}}"
if [[ -z "$SPACE_URL" ]]; then
  echo "usage: $0 <space-git-url>   (or set HF_SPACE_URL)" >&2
  exit 2
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND="$REPO_ROOT/apps/backend"
SPACE_README="$REPO_ROOT/deploy/huggingface/README.md"

for required in "$BACKEND/Dockerfile" "$BACKEND/go.mod" "$SPACE_README"; do
  [[ -f "$required" ]] || { echo "missing $required" >&2; exit 1; }
done

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo "→ cloning $SPACE_URL"
git clone --depth 1 "$SPACE_URL" "$WORK/space" 2>&1 | tail -2

cd "$WORK/space"
# Drop everything tracked except git itself, so a file deleted in apps/backend also disappears
# from the Space instead of lingering and being built.
git ls-files -z | xargs -0 -r rm -f

# The Dockerfile expects the backend directory as its build context, which is exactly what the
# Space root becomes.
cp -R "$BACKEND"/. .
rm -rf .env .env.* bin coverage.out ./*.exe
cp "$SPACE_README" README.md

git add -A
if git diff --cached --quiet; then
  echo "→ nothing changed"
  exit 0
fi

git -c user.name="tracking-cashflow sync" -c user.email="sync@localhost" \
  commit -q -m "Sync backend from $(cd "$REPO_ROOT" && git rev-parse --short HEAD)"
echo "→ pushing"
git push 2>&1 | tail -3
echo "→ done; the Space rebuilds automatically"
