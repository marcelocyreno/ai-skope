#!/bin/sh
# Mimics an opencode that carries its own credentials: `--version` for
# detection, `models` for the listing the server falls back to when no
# provider in the registry is scoped to this runtime.
if [ "$1" = "--version" ]; then echo "1.18.20"; exit 0; fi
if [ "$1" = "models" ]; then
	echo "zai-coding-plan/glm-4.7"
	echo "opencode/big-pickle"
	exit 0
fi
cat > /dev/null
echo '{"type":"text","part":{"type":"text","text":"hi"}}'
