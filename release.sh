#!/usr/bin/env bash
# release.sh vX.Y.Z — cut a release of this Go module.
# Blocks unless CI is green on the current HEAD. The git tag IS the module
# version (consumers: go get github.com/fromforgesoftware/go-kit@vX.Y.Z).
set -euo pipefail
REPO="fromforgesoftware/go-kit"
V="${1:?usage: ./release.sh vX.Y.Z}"
[[ "$V" == v*.*.* ]] || { echo "✗ version must look like v0.1.0"; exit 1; }

sha="$(git rev-parse HEAD)"
echo "→ verifying CI is green for $sha …"
concl="$(gh run list -R "$REPO" --workflow=CI --limit 30 \
  --json headSha,status,conclusion \
  -q "[.[] | select(.headSha==\"$sha\")][0] | .conclusion // .status // \"none\"")"
if [ "$concl" != "success" ]; then
  echo "✗ CI is '$concl' (not success) for HEAD — release blocked."
  echo "  push your commit, wait for CI to go green, then re-run."
  exit 1
fi

echo "→ tagging + releasing $V …"
git tag -a "$V" -m "$V"
git push origin "$V"
gh release create "$V" -R "$REPO" --generate-notes --title "$V"
echo "✓ released $V — go get github.com/fromforgesoftware/go-kit@$V"
