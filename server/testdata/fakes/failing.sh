#!/bin/sh
cat > /dev/null
echo "could not reach the provider: connection refused" >&2
exit 3
