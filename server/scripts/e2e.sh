#!/usr/bin/env bash
# End-to-end test: builds aiss, starts a real server on a scratch home, pairs a
# fake extension, allows a folder, indexes it, runs a chat turn through a fake
# agent, and checks every answer over HTTP — the same calls the extension makes.
#
#   ./scripts/e2e.sh
#
# Exits non-zero on the first failure and always stops the server it started.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
PORT="${AISS_E2E_PORT:-7377}"
BASE="http://127.0.0.1:${PORT}"
ORIGIN="chrome-extension://e2eextensionidaaaaaaaaaaaaaaaaaa"
PASS=0
FAIL=0

export XDG_CONFIG_HOME="$WORK/config"
export XDG_DATA_HOME="$WORK/data"
export XDG_STATE_HOME="$WORK/state"
export AISS_PORT="$PORT"
export AISS_KEYSTORE=file          # never touch the developer's real keychain

PROVIDER_STUB_PID=""
cleanup() {
  "$WORK/aiss" stop >/dev/null 2>&1 || true
  sleep 0.3
  pkill -f "$WORK/aiss" >/dev/null 2>&1 || true
  # The stub provider would otherwise outlive the run and keep the pipe open.
  # Silence the shell's own "Terminated" notice for the stub.
  if [ -n "$PROVIDER_STUB_PID" ]; then
    kill "$PROVIDER_STUB_PID" >/dev/null 2>&1
    wait "$PROVIDER_STUB_PID" 2>/dev/null
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT

say()  { printf '\n\033[1m%s\033[0m\n' "$*"; }
ok()   { PASS=$((PASS+1)); printf '  \033[32m✓\033[0m %s\n' "$*"; }
bad()  { FAIL=$((FAIL+1)); printf '  \033[31m✗\033[0m %s\n' "$*"; }
check() { # check <description> <condition-output>  — non-empty means pass
  if [ -n "$2" ]; then ok "$1"; else bad "$1"; fi
}
expect_eq() {
  if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2', want '$3')"; fi
}

api() { # api METHOD PATH [BODY]
  local method="$1" path="$2" body="${3:-}"
  if [ -n "$body" ]; then
    curl -sS -X "$method" "$BASE$path" -H "Authorization: Bearer $TOKEN" \
      -H "Origin: $ORIGIN" -H 'Content-Type: application/json' -d "$body"
  else
    curl -sS -X "$method" "$BASE$path" -H "Authorization: Bearer $TOKEN" -H "Origin: $ORIGIN"
  fi
}
code() { # code METHOD PATH [BODY] -> HTTP status only
  local method="$1" path="$2" body="${3:-}"
  if [ -n "$body" ]; then
    curl -sS -o /dev/null -w '%{http_code}' -X "$method" "$BASE$path" \
      -H "Authorization: Bearer ${TOKEN:-}" -H "Origin: $ORIGIN" -H 'Content-Type: application/json' -d "$body"
  else
    curl -sS -o /dev/null -w '%{http_code}' -X "$method" "$BASE$path" \
      -H "Authorization: Bearer ${TOKEN:-}" -H "Origin: $ORIGIN"
  fi
}
jq_() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)" 2>/dev/null; }

say "Build"
go build -o "$WORK/aiss" ./cmd/aiss || { bad "build"; exit 1; }
ok "aiss built ($("$WORK/aiss" version))"

say "Fixture: a project folder with a local page and notes"
PROJ="$WORK/dev/northwind"
mkdir -p "$PROJ/docs" "$PROJ/node_modules/dep"
# The server resolves symlinks (on macOS /var -> /private/var), so compare
# against the canonical path the way it will come back.
PROJ="$(cd "$PROJ" && pwd -P)"
cat > "$PROJ/README.md" <<'EOF'
# northwind

## Export format
Each statement month is written as CSV and JSON into ~/.northwind/export.
The JSON is the source of truth for re-imports.
EOF
cat > "$PROJ/docs/pricing.html" <<'EOF'
<html><head><title>Northwind Pricing</title></head><body>
<nav>Skip this navigation</nav><script>var tracking = 1;</script>
<h1>Plans</h1><p>Growth costs $149 per month and caps at 25M events.</p>
</body></html>
EOF
echo "SECRET_TOKEN=must-never-be-read" > "$PROJ/.env"
echo "module noise" > "$PROJ/node_modules/dep/index.js"
ok "fixture at $PROJ"

say "Start the server"
"$WORK/aiss" start >/dev/null 2>&1
for i in $(seq 1 50); do
  curl -sf "$BASE/v1/health" >/dev/null 2>&1 && break
  sleep 0.2
done
HEALTH="$(curl -sS "$BASE/v1/health")"
check "server answers /v1/health" "$(echo "$HEALTH" | jq_ 'd["status"]=="ok" or ""')"
expect_eq "health reports not yet paired" "$(echo "$HEALTH" | jq_ 'd["paired"]')" "False"

say "Auth"
expect_eq "unauthenticated request is refused" "$(TOKEN= code GET /v1/models)" "401"
CODE="$("$WORK/aiss" pair | head -1 | awk '{print $3}')"
check "aiss pair issued a code" "$CODE"
PAIRED="$(curl -sS -X POST "$BASE/v1/pair" -H "Origin: $ORIGIN" -H 'Content-Type: application/json' \
  -d "{\"code\":\"$CODE\",\"origin\":\"$ORIGIN\",\"label\":\"e2e\"}")"
TOKEN="$(echo "$PAIRED" | jq_ 'd["token"]')"
check "pairing returned a token" "$TOKEN"
expect_eq "the same code cannot be used twice" \
  "$(curl -sS -o /dev/null -w '%{http_code}' -X POST "$BASE/v1/pair" -H "Origin: $ORIGIN" \
     -H 'Content-Type: application/json' -d "{\"code\":\"$CODE\",\"origin\":\"$ORIGIN\"}")" "403"
expect_eq "a token from an unpaired origin is refused" \
  "$(curl -sS -o /dev/null -w '%{http_code}' "$BASE/v1/models" -H "Authorization: Bearer $TOKEN" \
     -H 'Origin: https://evil.example')" "403"
expect_eq "authenticated request now works" "$(code GET /v1/models)" "200"

say "Folders and indexing"
ADD="$(api POST /v1/folders "{\"path\":\"$PROJ\",\"access\":\"read+watch\"}")"
FOLDER_ID="$(echo "$ADD" | jq_ 'd["id"]')"
check "folder allowed" "$FOLDER_ID"
for i in $(seq 1 40); do
  HITS="$(api GET '/v1/files/search?q=export' | jq_ 'len(d["files"])')"
  [ "${HITS:-0}" -gt 0 ] && break
  sleep 0.25
done
expect_eq "the README is found by its content" \
  "$(api GET '/v1/files/search?q=export' | jq_ 'd["files"][0]["name"]')" "README.md"
expect_eq "the local HTML page is indexed as text" \
  "$(api GET '/v1/files/search?q=Growth' | jq_ 'd["files"][0]["name"]')" "pricing.html"
expect_eq "node_modules is not indexed" \
  "$(api GET '/v1/files/search?q=noise' | jq_ 'len(d["files"])')" "0"
expect_eq "the .env file is not indexed" \
  "$(api GET '/v1/files/search?q=must-never-be-read' | jq_ 'len(d["files"])')" "0"

say "Reading files"
READ="$(api GET "/v1/files/read?path=$PROJ/docs/pricing.html")"
check "HTML is returned as readable text" "$(echo "$READ" | jq_ '"yes" if "Growth costs" in d["text"] and "25M events" in d["text"] else ""')"
check "navigation and scripts are stripped" "$(echo "$READ" | jq_ '"" if "Skip this navigation" in d["text"] or "tracking" in d["text"] else "yes"')"
expect_eq "the page title is extracted" "$(echo "$READ" | jq_ 'd["title"]')" "Northwind Pricing"
expect_eq "reading .env is refused" "$(code GET "/v1/files/read?path=$PROJ/.env")" "403"
expect_eq "reading outside the folder is refused" "$(code GET '/v1/files/read?path=/etc/passwd')" "403"
expect_eq "escaping with .. is refused" "$(code GET "/v1/files/read?path=$PROJ/../../../etc/passwd")" "403"
expect_eq "a file:// URL inside the folder resolves" \
  "$(api POST /v1/files/resolve "{\"url\":\"file://$PROJ/README.md\"}" | jq_ 'd["path"]')" "$PROJ/README.md"
expect_eq "a file:// URL outside the folder is refused" \
  "$(code POST /v1/files/resolve '{"url":"file:///etc/passwd"}')" "403"

say "Runtimes and models"
"$WORK/aiss" runtimes command custom:e2e "$ROOT/testdata/fakes/claude-like.sh" >/dev/null
api POST /v1/runtimes/detect >/dev/null
check "the fake runtime is detected" \
  "$(api GET /v1/runtimes | jq_ '"yes" if any(r["id"]=="custom:e2e" and r["available"] for r in d["runtimes"]) else ""')"
api PUT /v1/models/default '{"runtime":"custom:e2e","model":"fake-1"}' >/dev/null
expect_eq "the default model is stored" \
  "$(api GET /v1/models | jq_ 'd["default"]["runtime"]')" "custom:e2e"

say "A chat turn, end to end"
CHAT_ID="$(api POST /v1/chats '{"url":"https://northwind.example/pricing","pageTitle":"Pricing"}' | jq_ 'd["id"]')"
check "chat created" "$CHAT_ID"
STREAM="$WORK/stream.txt"
curl -sS -N -X POST "$BASE/v1/chats/$CHAT_ID/messages" \
  -H "Authorization: Bearer $TOKEN" -H "Origin: $ORIGIN" -H 'Content-Type: application/json' \
  -d "{\"text\":\"Is Growth enough for 40M events?\",
       \"page\":{\"url\":\"https://northwind.example/pricing\",\"title\":\"Pricing\"},
       \"context\":[{\"type\":\"element\",\"selector\":\"article.pg-tier.featured\",\"text\":\"Growth \$149\"},
                    {\"type\":\"file\",\"path\":\"$PROJ/README.md\"}]}" > "$STREAM"
check "the turn streamed events"        "$(grep -c 'event: turn.start' "$STREAM" | grep -v '^0$')"
check "text arrived as deltas"          "$(grep -c 'event: text.delta' "$STREAM" | grep -v '^0$')"
check "a tool line was reported"        "$(grep -c 'event: tool' "$STREAM" | grep -v '^0$')"
check "usage was reported"              "$(grep -c 'event: usage' "$STREAM" | grep -v '^0$')"
check "the turn ended"                  "$(grep -c 'event: turn.end' "$STREAM" | grep -v '^0$')"
check "the answer text came through"    "$(grep -c 'Growth caps at 25M events' "$STREAM" | grep -v '^0$')"
check "the picked element reached the agent" "$(grep -c 'article.pg-tier.featured' "$STREAM" | grep -v '^0$')"
check "the local file was inlined for the agent" "$(grep -c 'source of truth for re-imports' "$STREAM" | grep -v '^0$')"

say "The transcript survives"
GOT="$(api GET "/v1/chats/$CHAT_ID")"
expect_eq "two messages are stored" "$(echo "$GOT" | jq_ 'len(d["messages"])')" "2"
expect_eq "the user message kept its context" "$(echo "$GOT" | jq_ 'len(d["messages"][0]["context"])')" "2"
check "the chat was titled from the question" "$(echo "$GOT" | jq_ '"yes" if d["chat"]["title"].startswith("Is Growth enough") else ""')"
check "token usage was recorded" "$(echo "$GOT" | jq_ 'd["messages"][1]["usage"]["inputTokens"] or ""')"
expect_eq "history lists it under the page" \
  "$(api GET '/v1/chats?url=https://northwind.example/pricing' | jq_ 'len(d["chats"])')" "1"
expect_eq "the attached file is now recent" \
  "$(api GET /v1/files/recent | jq_ 'd["files"][0]["name"]')" "README.md"

say "Delete and undo"
api DELETE "/v1/chats/$CHAT_ID" >/dev/null
expect_eq "a deleted chat is hidden" "$(api GET /v1/chats | jq_ 'len(d["chats"])')" "0"
api POST "/v1/chats/$CHAT_ID/restore" >/dev/null
expect_eq "undo restores it" "$(api GET /v1/chats | jq_ 'len(d["chats"])')" "1"

say "Notes"
NOTE_ID="$(api POST /v1/notes '{"url":"https://northwind.example/pricing","title":"Pricing","quote":"Growth is free for 12 months","body":"Ask finance about SAFEs"}' | jq_ 'd["id"]')"
check "note created" "$NOTE_ID"
expect_eq "notes are searchable" "$(api GET '/v1/notes?q=SAFEs' | jq_ 'len(d["notes"])')" "1"
api DELETE "/v1/notes/$NOTE_ID" >/dev/null
expect_eq "note deleted" "$(api GET /v1/notes | jq_ 'len(d["notes"])')" "0"

say "Providers (server-held keys)"
FAKE_API="$WORK/provider.log"
# The stub provider is written to a file rather than fed on stdin: a heredoc
# attached to a background job leaks its own text into this script's output.
cat > "$WORK/stub_provider.py" <<'PY'
import http.server, json, socketserver, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = json.dumps({"data":[{"id":"GLM 5.3","context_length":200000},{"id":"GLM 5.3 Flash"}]}).encode()
        self.send_response(200); self.send_header("Content-Type","application/json")
        self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)
    def log_message(self, *a): pass
with socketserver.TCPServer(("127.0.0.1", 0), H) as srv:
    open(sys.argv[1], "w").write(str(srv.server_address[1]))
    srv.serve_forever()
PY
python3 "$WORK/stub_provider.py" "$WORK/port.txt" >/dev/null 2>&1 &
PROVIDER_STUB_PID=$!
for i in $(seq 1 40); do [ -s "$WORK/port.txt" ] && break; sleep 0.1; done
PPORT="$(cat "$WORK/port.txt")"
PROV="$(api POST /v1/providers "{\"kind\":\"openai-compatible\",\"name\":\"z.ai\",\"baseUrl\":\"http://127.0.0.1:$PPORT\",\"key\":\"zai-secret-value\",\"availableTo\":[\"pi\",\"opencode\"]}")"
PROV_ID="$(echo "$PROV" | jq_ 'd["id"]')"
check "provider added" "$PROV_ID"
expect_eq "its models were discovered" "$(echo "$PROV" | jq_ 'len(d["models"])')" "2"
check "the key is masked in the API" "$(echo "$PROV" | jq_ '"yes" if "…" in d["key"] and "secret-value" not in d["key"] else ""')"
check "the plaintext key is not in the database" \
  "$(grep -c 'zai-secret-value' "$XDG_DATA_HOME/ai-skope/aiss.db" >/dev/null 2>&1 && echo "" || echo yes)"
api DELETE "/v1/providers/$PROV_ID" >/dev/null

say "Diagnostics"
check "aiss status reports running" "$("$WORK/aiss" status | grep -c 'running' | grep -v '^0$')"
check "aiss doctor runs"            "$("$WORK/aiss" doctor | grep -c 'AI Skope Server' | grep -v '^0$')"
check "aiss folders lists the folder" "$("$WORK/aiss" folders list | grep -c northwind | grep -v '^0$')"
check "logs are written"            "$("$WORK/aiss" logs --tail 5 | grep -c 'aiss listening' | grep -v '^0$')"

say "Shutdown"
"$WORK/aiss" stop >/dev/null 2>&1
sleep 0.5
expect_eq "the server is gone" "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/v1/health" 2>/dev/null)" "000"

# The privacy policy promises this command deletes everything the server holds,
# so the promise is checked rather than assumed.
say "Reset"
check "the database exists before the reset" "$([ -f "$XDG_DATA_HOME/ai-skope/aiss.db" ] && echo yes)"
"$WORK/aiss" reset --yes >/dev/null 2>&1
check "the data directory is gone"   "$([ ! -d "$XDG_DATA_HOME/ai-skope" ] && echo yes)"
check "the config directory is gone" "$([ ! -d "$XDG_CONFIG_HOME/ai-skope" ] && echo yes)"
check "the log directory is gone"    "$([ ! -d "$XDG_STATE_HOME/ai-skope" ] && echo yes)"

printf '\n\033[1m%d passed, %d failed\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
