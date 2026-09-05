#!/bin/sh
# The agent behind the store screenshots.
#
# The screenshots show the real extension, the real server and real streaming;
# only the agent is scripted, so the picture is the same every time it is shot
# and no live model bill is spent on marketing assets. The answer is written
# against the fixture page it is shown answering about.
cat > /dev/null
printf '%s\n' '{"type":"system","subtype":"init","session_id":"shot-1"}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"pricing.html"}}]}}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"**No — Growth would not cover 40M events.**\n\nGrowth is capped at `25M` events a month at $149, so 40M puts you over by 15M.\n\nWhat the page actually offers:\n\n- **Enterprise** is the only tier with custom volume, and it is priced on request\n- **Growth** stays viable only if you can cut ingest below the cap\n\nOne thing the page does not say: what happens when you cross the cap. There is no overage rate anywhere on it, so that is worth asking before you commit.\n\nSeats are not a factor either way — every tier includes unlimited seats and 13 months of history."}]}}'
printf '%s\n' '{"type":"result","subtype":"success","usage":{"input_tokens":2140,"output_tokens":137},"session_id":"shot-1"}'
