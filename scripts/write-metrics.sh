#!/usr/bin/env bash
set -euo pipefail

root=${1:?repository root is required}
work=${2:?work directory is required}
output=${3:?metrics output is required}

go_files=$(git -C "$root" ls-files '*.go' | wc -l | tr -d ' ')
gooo_files=$(git -C "$root" ls-files '*.gooo' | wc -l | tr -d ' ')
go_lines=$(git -C "$root" ls-files '*.go' -z | xargs -0 awk '{count += 1} END {print count + 0}')
gooo_lines=$(git -C "$root" ls-files '*.gooo' -z | xargs -0 awk '{count += 1} END {print count + 0}')
regular_files=$(git -C "$root" ls-files | awk '$0 != "README.md" {count += 1} END {print count + 0}')
files_total=$regular_files
subdirectories=$(git -C "$root" ls-files | awk -F/ 'NF > 1 {for (i=1; i<NF; i++) {path=""; for (j=1; j<=i; j++) path = path (j == 1 ? "" : "/") $j; seen[path]=1}} END {count=0; for (path in seen) count++; print count + 0}')
terminal_report="$work/terminal/conformance-report.json"
output_files=$(( $(jq -r '.output_files' "$work/conformance-report.json") + $(jq -r '.output_files' "$terminal_report") ))
output_bytes=$(( $(jq -r '.output_bytes' "$work/conformance-report.json") + $(jq -r '.output_bytes' "$terminal_report") ))
peak_rss=$(jq -n --argjson legacy "$(jq -r '.peak_rss_bytes' "$work/conformance-report.json")" --argjson terminal "$(jq -r '.peak_rss_bytes' "$terminal_report")" '$legacy | if $terminal > . then $terminal else . end')
generated_artifact_count=$(jq -r '.generated_artifacts.count' "$terminal_report")
generated_artifact_bytes=$(jq -r '.generated_artifacts.bytes' "$terminal_report")
integration_wall_ms=$(jq -r '.integration.wall_ms' "$work/improvement-report.json")
integration_peak_rss_kib=$(jq -r '.integration.peak_rss_kib' "$work/improvement-report.json")
improvement_status=$(jq -r '.status' "$work/improvement-report.json")
before_topology=$(jq -r '.resolution_pairs.supported_valid_topology_cardinalities.before' "$work/improvement-report.json")
after_topology=$(jq -r '.resolution_pairs.supported_valid_topology_cardinalities.after' "$work/improvement-report.json")
before_cases=$(jq -r '.resolution_pairs.accepted_trial_candidate_cases.before' "$work/improvement-report.json")
after_cases=$(jq -r '.resolution_pairs.accepted_trial_candidate_cases.after' "$work/improvement-report.json")
before_stages=$(jq -r '.resolution_pairs.coarse_localization_stages.before' "$work/improvement-report.json")
after_stages=$(jq -r '.resolution_pairs.coarse_localization_stages.after' "$work/improvement-report.json")
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
		--argjson files_total "$files_total" --argjson regular_files "$regular_files" --argjson subdirectories "$subdirectories" \
		--argjson output_files "$output_files" --argjson output_bytes "$output_bytes" \
		--argjson generated_artifact_count "$generated_artifact_count" --argjson generated_artifact_bytes "$generated_artifact_bytes" \
		--argjson peak_rss_bytes "$peak_rss" \
	--argjson integration_wall_ms "$integration_wall_ms" --argjson integration_peak_rss_kib "$integration_peak_rss_kib" \
	--argjson compile_wall_ms "$(jq -r '.compile_wall_ms' "$work/conformance-report.json")" \
	--argjson build_wall_ms "$(jq -r '.wall_ms' "$work/timing/build.json")" \
	--argjson test_wall_ms "$(jq -r '.wall_ms' "$work/timing/test.json")" \
		--argjson conformance_wall_ms "$(( $(jq -r '.conformance_wall_ms' "$work/conformance-report.json") + $(jq -r '.wall_ms' "$terminal_report") ))" \
		--argjson terminal_conformance_wall_ms "$(jq -r '.wall_ms' "$terminal_report")" \
		--argjson tests_total "$(jq -r '.tests.total' "$terminal_report")" \
		--argjson tests_selected "$(jq -r '.tests.selected' "$terminal_report")" \
		--argjson tests_executed "$(jq -r '.tests.executed' "$terminal_report")" \
		--argjson tests_reused "$(jq -r '.tests.reused' "$terminal_report")" \
		--argjson tests_failed "$(jq -r '.tests.failed' "$terminal_report")" \
		--argjson tests_unknown "$(jq -r '.tests.unknown' "$terminal_report")" \
	--arg improvement_status "$improvement_status" --argjson before_topology "$before_topology" --argjson after_topology "$after_topology" \
	--argjson before_cases "$before_cases" --argjson after_cases "$after_cases" --argjson before_stages "$before_stages" --argjson after_stages "$after_stages" \
		'{schema:$schema,inventory:{go_files:$go_files,gooo_files:$gooo_files,go_physical_lines:$go_physical_lines,gooo_physical_lines:$gooo_physical_lines,regular_files:$regular_files,files_total:$files_total,subdirectories:$subdirectories,root_readme_inventory_excluded:1},outputs:{count:$output_files,bytes:$output_bytes},generated_artifacts:{count:$generated_artifact_count,bytes:$generated_artifact_bytes},resources:{peak_rss_bytes:$peak_rss_bytes,peak_rss_kib:($peak_rss_bytes / 1024)},wall_ms:{compile:$compile_wall_ms,build:$build_wall_ms,test:$test_wall_ms,conformance:$conformance_wall_ms,terminal_conformance:$terminal_conformance_wall_ms,integration:$integration_wall_ms},tests:{total:$tests_total,selected:$tests_selected,executed:$tests_executed,reused:$tests_reused,failed:$tests_failed,unknown:$tests_unknown},improvement:{status:$improvement_status,resolution_pairs:{supported_valid_topology_cardinalities:{before:$before_topology,after:$after_topology},accepted_trial_candidate_cases:{before:$before_cases,after:$after_cases},coarse_localization_stages:{before:$before_stages,after:$after_stages}}}}' \
	> "$output"
