#!/usr/bin/env bash
set -euo pipefail

root=${1:?repository root is required}
work=${2:?work directory is required}
compiler=${3:?compiler executable is required}
verifier=${4:?verifier executable is required}
corpus="$root/contracts/terminal-corpus-v1.json"
terminal_work="$work/terminal"
mkdir -p "$terminal_work/runs" "$terminal_work/timing"

max_rss=0
run_timed() {
	local label=$1
	shift
	local timing="$terminal_work/timing/$label.txt"
	set +e
	/usr/bin/time -v -o "$timing" "$@"
	local status=$?
	set -e
	local rss
	rss=$(awk -F: '/Maximum resident set size/ {gsub(/^[ \t]+/, "", $2); print $2 * 1024}' "$timing")
	if [ "${rss:-0}" -gt "$max_rss" ]; then max_rss=$rss; fi
	return "$status"
}

started=$(date +%s%3N)
test_total=0
test_selected=0
test_unknown=0
test_failed=0
closed=0
unknown=0
refuted=0
case_total=$(jq '.cases | length' "$corpus")
topology_total=$(jq '.topologies | length' "$corpus")
topology_json='[]'

while IFS=$'\t' read -r topology_id phase expected_activities expected_stages; do
	[ -n "$topology_id" ] || continue
	local_total=0
	local_unknown=0
	local_closed=0
	local_refuted=0
	inspect="$terminal_work/topology-$topology_id.json"
	"$compiler" --inspect-phase --json --phase "$root/$phase" > "$inspect"
	jq -e --argjson activities "$expected_activities" --argjson stages "$expected_stages" \
		'.valid == true and .activity_count == $activities and .localization_stages == $stages' "$inspect" >/dev/null

	while IFS=$'\t' read -r case_id source expected expected_unknown_class; do
		[ -n "$case_id" ] || continue
		local_total=$((local_total + 1))
		test_total=$((test_total + 1))
		test_selected=$((test_selected + 1))
		case_dir="$terminal_work/runs/$topology_id/$case_id"
		baseline_dir="$case_dir/baseline"
		candidate_dir="$case_dir/candidate"
		mkdir -p "$baseline_dir" "$candidate_dir"
		if ! run_timed "${topology_id}-${case_id}-baseline" "$compiler" --phase "$root/$phase" \
			--input "$root/$source" --input-kind source --source "$root/$source" \
			--output-dir "$baseline_dir" --run-id "$topology_id-$case_id-baseline" --role baseline; then
			test_failed=$((test_failed + 1))
			continue
		fi
		if ! run_timed "${topology_id}-${case_id}-candidate" "$compiler" --phase "$root/$phase" \
			--input "$baseline_dir/semantic-ir.json" --input-kind semantic-ir --source "$root/$source" \
			--output-dir "$candidate_dir" --run-id "$topology_id-$case_id-candidate" --role candidate; then
			test_failed=$((test_failed + 1))
			continue
		fi
		if ! run_timed "${topology_id}-${case_id}-verify" "$verifier" --phase "$root/$phase" \
			--source "$root/$source" --baseline-dir "$baseline_dir" --candidate-dir "$candidate_dir" \
			--expected "$expected" --output "$case_dir/independent-verification.json"; then
			test_failed=$((test_failed + 1))
			continue
		fi
		actual=$(jq -r '.decision' "$case_dir/independent-verification.json")
		terminal=$(jq -r '.decision' "$baseline_dir/terminal-record.json")
		if [ "$actual" != "$expected" ] || [ "$terminal" != "$expected" ]; then
			test_failed=$((test_failed + 1))
			continue
		fi
		if [ "$expected" = "UNKNOWN" ] && [ "$(jq -r '.unknown_class' "$baseline_dir/terminal-record.json")" != "$expected_unknown_class" ]; then
			test_failed=$((test_failed + 1))
			continue
		fi
		case "$actual" in
		CLOSED) closed=$((closed + 1)); local_closed=$((local_closed + 1));;
		UNKNOWN) unknown=$((unknown + 1)); local_unknown=$((local_unknown + 1)); test_unknown=$((test_unknown + 1));;
		REFUTED) refuted=$((refuted + 1)); local_refuted=$((local_refuted + 1));;
		*) test_failed=$((test_failed + 1));;
		esac
	done < <(jq -r '.cases[] | [.id,.source,.expected,.unknown_class] | @tsv' "$corpus")
	topology_json=$(jq -c --arg id "$topology_id" --argjson total "$local_total" \
		--argjson closed "$local_closed" --argjson unknown "$local_unknown" --argjson refuted "$local_refuted" \
		'. + [{id:$id,tests:{total:$total,selected:$total,executed:$total,reused:0,failed:0,unknown:$unknown},cases:{closed:$closed,unknown:$unknown,refuted:$refuted}}]' \
		<<< "$topology_json")
done < <(jq -r '.topologies[] | [.id,.phase,.activity_count,.localization_stages] | @tsv' "$corpus")

ended=$(date +%s%3N)
integration_wall_ms=$((ended - started))
if [ "$integration_wall_ms" -lt 1 ]; then integration_wall_ms=1; fi
output_files=$(find "$terminal_work/runs" -type f | wc -l | tr -d ' ')
output_bytes=$(find "$terminal_work/runs" -type f -print0 | xargs -0 stat -c '%s' | awk '{total += $1} END {print total + 0}')
generated_artifact_count=$(find "$terminal_work/runs" -type f \( -name 'semantic-ir.json' -o -name 'generated.go' -o -name 'terminal-record.json' \) | wc -l | tr -d ' ')
generated_artifact_bytes=$(find "$terminal_work/runs" -type f \( -name 'semantic-ir.json' -o -name 'generated.go' -o -name 'terminal-record.json' \) -print0 | xargs -0 stat -c '%s' | awk '{total += $1} END {print total + 0}')

jq -n --arg schema "gooo/reflexive-terminal-conformance/v1" \
	--argjson total "$test_total" --argjson selected "$test_selected" --argjson executed "$((test_total - test_failed))" \
	--argjson reused 0 --argjson failed "$test_failed" --argjson unknown "$test_unknown" \
	--argjson cases "$case_total" --argjson topologies "$topology_total" \
	--argjson closed "$closed" --argjson refuted "$refuted" --argjson unknown_cases "$unknown" \
	--argjson wall_ms "$integration_wall_ms" --argjson peak_rss_bytes "$max_rss" \
	--argjson output_files "$output_files" --argjson output_bytes "$output_bytes" \
	--argjson generated_count "$generated_artifact_count" --argjson generated_bytes "$generated_artifact_bytes" \
	--argjson topology_runs "$topology_json" \
	'{schema:$schema,decision:(if $failed == 0 and $cases == 9 and $topologies == 2 and $closed == 4 and $unknown_cases == 10 and $refuted == 4 then "CLOSED" else "FAIL_CLOSED" end),tests:{total:$total,selected:$selected,executed:$executed,reused:$reused,failed:$failed,unknown:$unknown},cases:{total:$cases,closed:($closed / $topologies),unknown:($unknown_cases / $topologies),refuted:($refuted / $topologies)},topologies:$topology_runs,wall_ms:$wall_ms,peak_rss_bytes:$peak_rss_bytes,output_files:$output_files,output_bytes:$output_bytes,generated_artifacts:{count:$generated_count,bytes:$generated_bytes}}' \
	> "$terminal_work/conformance-report.json"

jq -e '
	.decision == "CLOSED" and .tests == {total:18,selected:18,executed:18,reused:0,failed:0,unknown:10} and
	.cases == {total:9,closed:2,unknown:5,refuted:2} and
	([.topologies[] | .tests.total] | sort) == [9,9] and
	([.topologies[] | .cases] | sort_by(.closed,.unknown,.refuted)) == [{closed:2,unknown:5,refuted:2},{closed:2,unknown:5,refuted:2}]
' "$terminal_work/conformance-report.json" >/dev/null
