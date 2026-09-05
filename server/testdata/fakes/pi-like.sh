#!/bin/sh
# Mimics `pi -p --mode json` and `omp -p --mode json`: a session frame whose id
# is plain "id", text arriving inside assistantMessageEvent, thinking that must
# not be shown, and usage reported only on the finished message.
cat > /dev/null
echo '{"type":"session","version":3,"id":"01a06f28-pi-session","cwd":"/tmp"}'
echo '{"type":"agent_start"}'
echo '{"type":"message_start","message":{"role":"user","content":[{"type":"text","text":"THE USER PROMPT"}]}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"thinking_start","contentIndex":0},"usage":{"input":0,"output":0}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":"SECRET REASONING"},"usage":{"input":0,"output":0}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"text_start","contentIndex":1}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":1,"delta":"one "}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":1,"delta":"two"}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"text_end","contentIndex":1,"content":"one two"}}'
echo '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"one two"}]}}'
echo '{"type":"turn_end","message":{"role":"assistant","content":[{"type":"text","text":"one two"}],"usage":{"input":2044,"output":54,"totalTokens":2098}}}'
echo '{"type":"agent_end"}'
