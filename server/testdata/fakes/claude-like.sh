#!/bin/sh
# Emits Claude Code's stream-json shape, including a tool use and a result.
prompt=$(cat)
echo '{"type":"system","subtype":"init","session_id":"sess-123"}'
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/tmp/x/README.md"}}]}}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Growth caps at 25M events."}]}}'
echo "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\" You asked: ${prompt}\"}]}}"
echo '{"type":"result","subtype":"success","usage":{"input_tokens":1200,"output_tokens":48},"session_id":"sess-123"}'
