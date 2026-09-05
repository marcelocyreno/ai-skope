#!/bin/sh
# Emits Claude Code's stream-json shape, including a tool use and a result.
# Flattened: the prompt carries newlines and quotes, and this line has to
# stay one valid JSON object.
prompt=$(cat | tr -d '"\\' | tr '\n' ' ')
printf '%s\n' '{"type":"system","subtype":"init","session_id":"sess-123"}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/tmp/x/README.md"}}]}}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"Growth caps at 25M events."}]}}'
printf '%s\n' "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\" You asked: ${prompt}\"}]}}"
# A Markdown block, because that is how real agents answer.
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"\n\n**Limits**\n\n- Growth: `25M` events\n- Scale: 100M events\n"}]}}'
printf '%s\n' '{"type":"result","subtype":"success","usage":{"input_tokens":1200,"output_tokens":48},"session_id":"sess-123"}'
