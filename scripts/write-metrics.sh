#!/usr/bin/env bash
set -euo pipefail

root=${1:?repository root is required}
work=${2:?work directory is required}
output=${3:?metrics output is required}

go_files=$(git -C "$root" ls-files '*.go' | wc -l | tr -d ' ')
gooo_files=$(git -C "$root" ls-files '*.gooo' | wc -l | tr -d ' ')
go_lines=$(git -C "$root" ls-files '*.go' -z | xargs -0 awk '{count += 1} END {print count + 0}')
gooo_lines=$(git -C "$root" ls-files '*.gooo' -z | xargs -0 awk '{count += 1} END {print count + 0}')
files_total=$(git -C "$root" ls-files | awk '$0 != "README.md" {count += 1} END {print count + 0}')
subdirectories=$(git -C "$root" ls-files | awk -F/ 'NF > 1 {for (i=1; i<NF; i++) {path=""; for (j=1; j<=i; j++) path = path (j == 1 ? "" : "/") $j; seen[path]=1}} END {count=0; for (path in seen) count++; print count + 0}')
output_files=$(jq -r '.output_files' "$work/conformance-report.json")
output_bytes=$(jq -r '.output_bytes' "$work/conformance-report.json")
peak_rss=$(jq -r '.peak_rss_bytes' "$work/conformance-report.json")
for timing in build test; do
	if [ ! -f "$work/timing/$timing.json" ]; then
		echo "missing $timing timing" >&2
		exit 1
	fi
done
jq -n \
	--arg schema "gooo/reflexive-ci-metrics/v1" \
	--argjson go_files "$go_files" --argjson gooo_files "$gooo_files" \
	--argjson go_physical_lines "$go_lines" --argjson gooo_physical_lines "$gooo_lines" \
	--argjson files_total "$files_total" --argjson subdirectories "$subdirectories" \
	--argjson output_files "$output_files" --argjson output_bytes "$output_bytes" \
	--argjson peak_rss_bytes "$peak_rss" \
	--argjson compile_wall_ms "$(jq -r '.compile_wall_ms' "$work/conformance-report.json")" \
	--argjson build_wall_ms "$(jq -r '.wall_ms' "$work/timing/build.json")" \
	--argjson test_wall_ms "$(jq -r '.wall_ms' "$work/timing/test.json")" \
	--argjson conformance_wall_ms "$(jq -r '.conformance_wall_ms' "$work/conformance-report.json")" \
	--argjson tests_total "$(jq -r '.tests.total' "$work/conformance-report.json")" \
	--argjson tests_executed "$(jq -r '.tests.executed' "$work/conformance-report.json")" \
	--argjson tests_reused "$(jq -r '.tests.reused' "$work/conformance-report.json")" \
	--argjson tests_failed "$(jq -r '.tests.failed' "$work/conformance-report.json")" \
	--argjson tests_unknown "$(jq -r '.tests.unknown' "$work/conformance-report.json")" \
	'{schema:$schema,inventory:{go_files:$go_files,gooo_files:$gooo_files,go_physical_lines:$go_physical_lines,gooo_physical_lines:$gooo_physical_lines,files_total:$files_total,subdirectories:$subdirectories,root_readme_inventory_excluded:1},outputs:{count:$output_files,bytes:$output_bytes},resources:{peak_rss_bytes:$peak_rss},wall_ms:{compile:$compile_wall_ms,build:$build_wall_ms,test:$test_wall_ms,conformance:$conformance_wall_ms},tests:{total:$tests_total,executed:$tests_executed,reused:$tests_reused,failed:$tests_failed,unknown:$tests_unknown}}' \
	> "$output"
