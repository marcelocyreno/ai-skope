#!/usr/bin/env bash
# Drives a *real* coding agent end to end through the server, exactly as the
# extension would: pair, allow a folder, attach a picked element and a local
# file, stream the answer, then ask a follow-up in the same agent session.
#
#   ./scripts/real-agent.sh claude-code sonnet            # runtime model
#   ./scripts/real-agent.sh pi glm-5.3-flash zai low      # runtime model provider effort
#
# Unlike scripts/e2e.sh this one calls a real model, so it needs the agent
# installed and authenticated, and it costs tokens.
set -uo pipefail

RUNTIME="${1:?usage: real-agent.sh <runtime> <model> [provider] [effort]}"
MODEL="${2:?usage: real-agent.sh <runtime> <model> [provider] [effort]}"
PROVIDER="${3:-}"
EFFORT="${4:-}"

W=$(mktemp -d); PORT="${AISS_E2E_PORT:-7466}"; BASE="http://127.0.0.1:$PORT"
ORIGIN="chrome-extension://realagenttest"
export XDG_CONFIG_HOME=$W/c XDG_DATA_HOME=$W/d XDG_STATE_HOME=$W/s AISS_PORT=$PORT AISS_KEYSTORE=file
cleanup() { "$W/aiss" stop >/dev/null 2>&1; rm -rf "$W"; }
trap cleanup EXIT

PASS=0; FAIL=0
ok(){ PASS=$((PASS+1)); printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad(){ FAIL=$((FAIL+1)); printf '  \033[31m✗\033[0m %s\n' "$1"; }
check(){ [ -n "$2" ] && ok "$1" || bad "$1"; }
jq_(){ python3 -c "import sys,json;d=json.load(sys.stdin);print($1)" 2>/dev/null; }

echo "Building"; go build -o "$W/aiss" ./cmd/aiss || exit 1

PROJ="$W/dev/finme"; mkdir -p "$PROJ"; PROJ="$(cd "$PROJ" && pwd -P)"
cat > "$PROJ/README.md" <<'EOF'
# finme

## Export format
Each statement month is written as CSV and JSON into ~/.finme/export.
The JSON is the source of truth for re-imports; the CSV is for spreadsheets.
EOF

"$W/aiss" start >/dev/null 2>&1
for i in $(seq 1 60); do curl -sf "$BASE/v1/health" >/dev/null 2>&1 && break; sleep 0.2; done
"$W/aiss" folders add "$PROJ" >/dev/null
CODE=$("$W/aiss" pair | head -1 | awk '{print $3}')
TOKEN=$(curl -sS -X POST "$BASE/v1/pair" -H 'Content-Type: application/json' \
  -d "{\"code\":\"$CODE\",\"origin\":\"$ORIGIN\"}" | jq_ 'd["token"]')
api(){ curl -sS -X "$1" "$BASE$2" -H "Authorization: Bearer $TOKEN" -H "Origin: $ORIGIN" \
        -H 'Content-Type: application/json' ${3:+-d "$3"}; }

echo; echo "Runtime: $RUNTIME"
api POST /v1/runtimes/detect >/dev/null
INFO=$(api GET /v1/runtimes | jq_ "[r for r in d['runtimes'] if r['id']=='$RUNTIME'][0]")
echo "  $INFO"
check "$RUNTIME is installed and healthy" \
  "$(api GET /v1/runtimes | jq_ "\"y\" if any(r['id']=='$RUNTIME' and r['available'] and r['status']=='ok' for r in d['runtimes']) else \"\"")"

SEL=$(python3 - "$RUNTIME" "$MODEL" "$PROVIDER" "$EFFORT" <<'PY'
import json,sys
r,m,p,e = sys.argv[1:5]
sel = {"runtime": r, "model": m}
if p: sel["provider"] = p
if e: sel["effort"] = e
print(json.dumps(sel))
PY
)
api PUT /v1/models/default "$SEL" >/dev/null
echo "  selection: $SEL"

echo; echo "A real turn"
CHAT=$(api POST /v1/chats '{"url":"https://northwind.example/pricing","pageTitle":"Pricing"}' | jq_ 'd["id"]')
BODY=$(python3 - "$PROJ" <<'PY'
import json,sys
print(json.dumps({
 "text":"Using ONLY the attached context, answer in one short sentence: which file format is the source of truth for re-imports, and what does the pricing element say Growth costs?",
 "page":{"url":"https://northwind.example/pricing","title":"Northwind Pricing"},
 "context":[
   {"type":"element","selector":"article.pg-tier.featured","text":"Growth $149 per month, capped at 25M events","rect":[320,412]},
   {"type":"file","path":sys.argv[1]+"/README.md"}
 ]}))
PY
)
START=$(date +%s)
curl -sS -N -X POST "$BASE/v1/chats/$CHAT/messages" -H "Authorization: Bearer $TOKEN" \
  -H "Origin: $ORIGIN" -H 'Content-Type: application/json' -d "$BODY" > "$W/stream.txt"
echo "  (took $(( $(date +%s) - START ))s)"

DELTAS=$(grep -c 'event: text.delta' "$W/stream.txt")
check "the turn streamed and ended"  "$(grep -c 'event: turn.end' "$W/stream.txt" | grep -v '^0$')"
check "the answer reached the client ($DELTAS delta(s))" "$([ "$DELTAS" -ge 1 ] && echo y)"
if [ "$DELTAS" -gt 3 ]; then ok "it streamed token by token"; else
  printf '  \033[33m~\033[0m %s\n' "this agent returns the answer in one piece, not token by token"
fi
check "no error event"              "$(grep -c 'event: error' "$W/stream.txt" | grep '^0$')"

ANSWER=$(python3 - "$W/stream.txt" <<'PY'
import json,sys
out=[]
for line in open(sys.argv[1]):
    if line.startswith("data: "):
        try: d=json.loads(line[6:])
        except Exception: continue
        if d.get("event")=="text.delta": out.append(d.get("text",""))
print("".join(out).strip())
PY
)
echo "  answer: $ANSWER"
check "the answer used the attached file (JSON)"      "$(echo "$ANSWER" | grep -ci 'json' | grep -v '^0$')"
check "the answer used the picked element (\$149)"    "$(echo "$ANSWER" | grep -c '149' | grep -v '^0$')"

GOT=$(api GET "/v1/chats/$CHAT")
check "the answer was stored exactly once" "$(python3 - <<PY
import json
d=json.loads('''$GOT''')
t=d["messages"][1]["text"].strip()
first=t.split(".")[0]
print("y" if first and t.count(first)==1 else "")
PY
)"
check "usage was recorded"  "$(echo "$GOT" | jq_ 'd["messages"][1]["usage"]["inputTokens"] or ""')"

echo; echo "Follow-up in the same agent session"
curl -sS -N -X POST "$BASE/v1/chats/$CHAT/messages" -H "Authorization: Bearer $TOKEN" \
  -H "Origin: $ORIGIN" -H 'Content-Type: application/json' \
  -d '{"text":"In one word: what did I just ask about?"}' > "$W/stream2.txt"
check "the follow-up answered"   "$(grep -c 'event: turn.end' "$W/stream2.txt" | grep -v '^0$')"
check "four messages are stored" "$(api GET "/v1/chats/$CHAT" | jq_ '"y" if len(d["messages"])==4 else ""')"

printf '\n\033[1m%s: %d passed, %d failed\033[0m\n' "$RUNTIME/$MODEL" "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
