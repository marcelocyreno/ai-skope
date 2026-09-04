#!/bin/sh
cat > /dev/null
env | sort | while read -r line; do
  printf '{"type":"assistant","message":{"content":[{"type":"text","text":"%s\\n"}]}}\n' "$line"
done
