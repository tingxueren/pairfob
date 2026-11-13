#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
test -f .cloudflare/pairfob-fork/wrangler.generated.json
# Keep live runtime secrets on Cloudflare; never generate replacements.
# A D1 lease covers final asset restoration, migrations and publication.
bun .cloudflare/pairfob-fork/lease.ts
