#!/bin/sh
# Mimics `claude -p --output-format stream-json --include-partial-messages`:
# token deltas, then the assembled assistant message, then the result — all
# carrying the same answer.
cat > /dev/null
echo '{"type":"system","subtype":"init","session_id":"sess-partial"}'
echo '{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}},"session_id":"sess-partial"}'
echo '{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"one "}},"session_id":"sess-partial"}'
echo '{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"two "}},"session_id":"sess-partial"}'
echo '{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"three"}},"session_id":"sess-partial"}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"one two three"}]},"session_id":"sess-partial"}'
echo '{"type":"rate_limit_event","session_id":"sess-partial"}'
echo '{"type":"result","subtype":"success","result":"one two three","usage":{"input_tokens":12,"output_tokens":3},"session_id":"sess-partial"}'
