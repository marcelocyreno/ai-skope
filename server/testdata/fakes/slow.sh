#!/bin/sh
cat > /dev/null
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"starting"}]}}'
sleep 30
echo '{"type":"result"}'
