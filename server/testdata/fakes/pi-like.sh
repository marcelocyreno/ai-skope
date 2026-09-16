#!/bin/sh
# Mimics `pi -p --mode json` and `omp -p --mode json`: a session frame whose id
# is plain "id", text arriving inside assistantMessageEvent, thinking that must
# not be shown, and usage reported only on the finished message.
#
# A tool call is announced in three assistantMessageEvent frames — start names
# the tool and the call, delta is the argument JSON arriving in pieces, end
# carries the assembled call — and then run, which pi reports at the top level
# as tool_execution_*. All six frames describe one call and must read as one
# row.
cat > /dev/null
echo '{"type":"session","version":3,"id":"01a06f28-pi-session","cwd":"/tmp"}'
echo '{"type":"agent_start"}'
echo '{"type":"message_start","message":{"role":"user","content":[{"type":"text","text":"THE USER PROMPT"}]}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"thinking_start","contentIndex":0},"usage":{"input":0,"output":0}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","contentIndex":0,"delta":"SECRET REASONING"},"usage":{"input":0,"output":0}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"toolcall_start","contentIndex":1,"id":"call_fake_1","toolName":"read"}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"toolcall_delta","contentIndex":1,"delta":"{\"path\":\"RE"}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"toolcall_end","contentIndex":1,"toolCall":{"type":"toolCall","id":"call_fake_1","name":"read","arguments":{"path":"README.md"}}}}'
echo '{"type":"tool_execution_start","toolCallId":"call_fake_1","toolName":"read","args":{"path":"README.md"}}'
echo '{"type":"tool_execution_end","toolCallId":"call_fake_1","toolName":"read","result":"# AI Skope","isError":false}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"text_start","contentIndex":1}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":1,"delta":"one "}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":1,"delta":"two"}}'
echo '{"type":"message_update","assistantMessageEvent":{"type":"text_end","contentIndex":1,"content":"one two"}}'
echo '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"one two"}]}}'
echo '{"type":"turn_end","message":{"role":"assistant","content":[{"type":"text","text":"one two"}],"usage":{"input":2044,"output":54,"totalTokens":2098}}}'
echo '{"type":"agent_end"}'
