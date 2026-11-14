#!/usr/bin/env bash
# Private real-node acceptance. Run from the operator Mac with --runner dorm-dev;
# only read-only collection/transfer runs there. Compilation/browser stay Linux.
set -euo pipefail
umask 077
cd "$(dirname "$0")/.."
repo="$PWD"
nodes="m3=local,dorm-dev=dorm-dev,nerd=nerd,mini=mini"
runner=""
input=""
while (($#)); do
  case "$1" in
    --nodes) nodes="${2:?missing nodes}"; shift 2 ;;
    --runner) runner="${2:?missing runner}"; shift 2 ;;
    --input) input="${2:?missing input directory}"; shift 2 ;;
    -h|--help)
      cat <<'HELP'
Usage: bash scripts/verify-core-ui.sh --runner dorm-dev
       bash scripts/verify-core-ui.sh --input .dev/core-ui/<run>
Options:
  --nodes name=local,name=ssh-alias,...  Capture nodes (default: all four).
  --runner ssh-alias                    Build/render on that node; rsync reports back.
  --input .dev/core-ui/<run>             Replay existing private captures on Linux.
Environment:
  PAIRFOB_EXPECT_BINARY_COMMIT           Require the deployed daemon commit.
  PAIRFOB_PLAYWRIGHT_MODULE              Installed Playwright module directory.
  PAIRFOB_CORE_UI_PORT                  Dedicated loopback Vite port (default 5197).
No dependency installation, deployment, pairing, terminal input or Core restart.
HELP
      exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done
if [[ -z "$input" ]]; then
  input=".dev/core-ui/$(date -u +%Y%m%dT%H%M%SZ)-$$"
  mkdir -p "$input/captures"
  IFS=',' read -ra targets <<< "$nodes"
  for target in "${targets[@]}"; do
    name="${target%%=*}"; host="${target#*=}"
    [[ "$target" == *=* && "$name" =~ ^[a-zA-Z0-9._-]+$ && "$host" =~ ^[a-zA-Z0-9._-]+$ ]] || { echo 'Invalid capture alias' >&2; exit 2; }
    echo "Read-only capture: $name ($host)"
    if [[ "$host" == local ]]; then
      PAIRFOB_ACCEPT_NODE="$name" node scripts/core-ui/capture.cjs > "$input/captures/$name.json"
    else
      ssh -o BatchMode=yes -o ConnectTimeout=15 "$host" \
        "PATH=\"\$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:\$PATH\" PAIRFOB_ACCEPT_NODE=$name node -" \
        < scripts/core-ui/capture.cjs > "$input/captures/$name.json"
    fi
  done
fi
[[ "$input" =~ ^\.dev/core-ui/[a-zA-Z0-9._-]+$ ]] || { echo 'Input must be .dev/core-ui/<safe-run-name>' >&2; exit 2; }
if [[ -n "$runner" ]]; then
  [[ "$runner" =~ ^[a-zA-Z0-9._-]+$ ]] || exit 2
  # Refuse to replay against a different script/source revision.
  local_tree=$(node scripts/core-ui/source-digest.cjs)
  remote_tree=$(ssh "$runner" 'cd "$HOME/Work/Git/pairfob" && node scripts/core-ui/source-digest.cjs')
  [[ "$local_tree" == "$remote_tree" ]] || { echo 'Runner source differs; sync the checkout first' >&2; exit 1; }
  ssh "$runner" "umask 077; mkdir -p \"\$HOME/Work/Git/pairfob/$input\""
  rsync -avP "$input/" "$runner:Work/Git/pairfob/$input/"
  expected_commit="${PAIRFOB_EXPECT_BINARY_COMMIT:-}"
  [[ -z "$expected_commit" || "$expected_commit" =~ ^[a-f0-9]{7,40}$ ]] || { echo 'Invalid expected binary commit' >&2; exit 2; }
  result=0
  ssh "$runner" "cd \"\$HOME/Work/Git/pairfob\" && PAIRFOB_EXPECT_BINARY_COMMIT=$expected_commit bash scripts/verify-core-ui.sh --input $input" || result=$?
  rsync -avP "$runner:Work/Git/pairfob/$input/" "$input/"
  exit "$result"
fi
# Replace any previous verdict before replay; an early failure must not leave
# a stale green report from an earlier candidate.
node - "$input" <<'JSSTART'
const fs=require('node:fs'),path=require('node:path');
fs.writeFileSync(path.join(process.argv[2],'report.json'),JSON.stringify({passed:false,status:'running',startedAt:new Date().toISOString()}),{mode:0o600});
JSSTART
vite_pid=""
finish() {
  local result=$?
  if [[ -n "$vite_pid" ]]; then
    kill "$vite_pid" 2>/dev/null || true
    wait "$vite_pid" 2>/dev/null || true
  fi
  if [[ "$result" != 0 ]]; then
    node - "$input" "$result" <<'JSFAIL'
const fs=require('node:fs'),path=require('node:path');
const p=path.join(process.argv[2],'report.json');
const report=JSON.parse(fs.readFileSync(p));
report.passed=false;report.status='failed';report.exitCode=Number(process.argv[3]);
fs.writeFileSync(p,JSON.stringify(report,null,2)+'\n',{mode:0o600});
JSFAIL
  fi
  exit "$result"
}
trap finish EXIT
[[ "$(uname -s)" == Linux ]] || { echo 'Use --runner dorm-dev; compile and render on Linux' >&2; exit 2; }
export PATH="$HOME/.bun/bin:$HOME/.cargo/bin:$PATH"
export GOTOOLCHAIN=go1.26.11
export PAIRFOB_CORE_UI_INPUT="$repo/$input"
export PAIRFOB_CORE_UI_SOURCE=$(node scripts/core-ui/source-digest.cjs)
node scripts/core-ui/check-input.cjs "$input"
go test ./internal/runtime -run '^TestCoreUIAcceptanceReplay$' -count=1 -v
node scripts/core-ui/selfcheck.cjs "$input"
port="${PAIRFOB_CORE_UI_PORT:-5197}"
[[ "$port" =~ ^[0-9]+$ ]] || exit 2
export PAIRFOB_CORE_UI_URL="http://127.0.0.1:$port"
# Own a new strict-port dev server; never kill/reuse someone else's Vite.
(cd pwa && exec node node_modules/vite/bin/vite.js --host 127.0.0.1 --port "$port" --strictPort) > "$input/vite.log" 2>&1 &
vite_pid=$!
ready=0
for ((attempt=0; attempt<100; attempt++)); do
  kill -0 "$vite_pid" 2>/dev/null || { cat "$input/vite.log"; exit 1; }
  if curl --silent --fail "$PAIRFOB_CORE_UI_URL/qa/index.html" > /dev/null; then ready=1; break; fi
  sleep 0.2
done
[[ "$ready" == 1 ]] || { echo 'Vite not ready' >&2; exit 1; }
node scripts/core-ui/browser.cjs "$input"

