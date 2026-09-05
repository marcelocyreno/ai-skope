#!/bin/sh
# Mimics `opencode run --format json`: parts carrying the text, a sessionID in
# that exact casing, and reasoning that must not reach the answer.
cat > /dev/null
echo '{"type":"step_start","sessionID":"ses_opencode123"}'
echo '{"type":"reasoning","sessionID":"ses_opencode123","part":{"type":"reasoning","text":"SECRET REASONING"}}'
echo '{"type":"text","sessionID":"ses_opencode123","part":{"id":"prt_1","type":"text","text":"one two"}}'
echo '{"type":"step_finish","sessionID":"ses_opencode123","part":{"type":"step-finish","reason":"stop","tokens":{"total":9354,"input":11,"output":4,"reasoning":0,"cache":{"write":0,"read":9344}}}}'
