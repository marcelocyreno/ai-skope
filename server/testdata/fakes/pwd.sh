#!/bin/sh
# Reports the directory it was run in, then echoes its prompt, so a test can
# check where the server put the agent as well as what it was told.
prompt=$(cat | tr -d '"\\' | tr '\n' ' ')
printf '{"type":"assistant","message":{"content":[{"type":"text","text":"cwd=%s "}]}}\n' "$(pwd -P)"
printf '{"type":"assistant","message":{"content":[{"type":"text","text":"prompt: %s"}]}}\n' "$prompt"
printf '%s\n' '{"type":"result","subtype":"success","usage":{"input_tokens":1,"output_tokens":1}}'
