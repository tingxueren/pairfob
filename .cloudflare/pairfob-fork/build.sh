#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
test "$(bun --version)" = "$(cat .bun-version)"
for directory in pwa workers/pairfob-origin workers/pairfob-downloads site/doc; do
  (cd "$directory" && bun install --frozen-lockfile)
done
# Run the existing web gate before preparing the only deployable artifact.
bash scripts/verify-web.sh
bun test scripts/selfhost-config.test.ts scripts/selfhost-pack.test.ts scripts/retain-pwa-assets.test.ts ./.cloudflare/pairfob-fork/lease.test.ts
bun .cloudflare/pairfob-fork/prepare.ts
workers/pairfob-origin/node_modules/.bin/wrangler deploy \
  --config .cloudflare/pairfob-fork/wrangler.generated.json --dry-run
