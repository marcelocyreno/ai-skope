#!/bin/sh
if [ "$1" = "--version" ]; then echo "fakeagent 1.4.2 (build 77)"; exit 0; fi
cat > /dev/null
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}'
