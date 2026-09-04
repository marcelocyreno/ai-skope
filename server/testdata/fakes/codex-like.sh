#!/bin/sh
# Emits Codex's item envelope, plus a legacy msg-wrapped event.
cat > /dev/null
echo '{"type":"thread.started","thread_id":"th-9"}'
echo '{"type":"item.started","item":{"type":"command_execution","command":"grep -n pricing"}}'
echo '{"type":"item.completed","item":{"type":"command_execution","command":"grep -n pricing"}}'
echo '{"msg":{"type":"agent_message","message":"Two conditions apply."}}'
echo '{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5}}'
