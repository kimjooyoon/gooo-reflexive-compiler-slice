#!/usr/bin/env bash
set -euo pipefail

output=${1:?timing output is required}
shift
if [ "${1:-}" != "--" ]; then
	echo "usage: measure-command.sh output -- command [args...]" >&2
	exit 2
fi
shift
start=$(date +%s%3N)
set +e
"$@"
status=$?
set -e
end=$(date +%s%3N)
wall=$((end - start))
if [ "$wall" -lt 1 ]; then wall=1; fi
mkdir -p "$(dirname "$output")"
jq -n --argjson wall_ms "$wall" --argjson exit_code "$status" '{wall_ms:$wall_ms,exit_code:$exit_code}' > "$output"
exit "$status"
