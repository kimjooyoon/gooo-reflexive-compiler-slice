#!/usr/bin/env bash
set -euo pipefail

root=${1:?repository root is required}
work=${2:?work directory is required}
compiler=${3:?compiler executable is required}
verifier=${4:?verifier executable is required}
mkdir -p "$work/runs" "$work/timing"

bash "$root/scripts/verify-upstream.sh" "$root" "$work"

max_rss=0
run_timed() {
	local label=$1
	shift
	local timing="$work/timing/$label.txt"
	set +e
	/usr/bin/time -v -o "$timing" "$@"
	local status=$?
	set -e
	local rss
	rss=$(awk -F: '/Maximum resident set size/ {gsub(/^[ \t]+/, "", $2); print $2 * 1024}' "$timing")
	if [ "${rss:-0}" -gt "$max_rss" ]; then
		max_rss=$rss
	fi
	return "$status"
}

compile_start=$(date +%s%3N)
case_count=0
unknown_count=0
refuted_count=0
closed_count=0
failed_count=0
while IFS=$'\t' read -r case_id source expected; do
	if [ -z "$case_id" ]; then
		continue
	fi
	case_count=$((case_count + 1))
	case_dir="$work/runs/$case_id"
	baseline_dir="$case_dir/baseline"
	candidate_dir="$case_dir/candidate"
	mkdir -p "$baseline_dir" "$candidate_dir"
	if ! run_timed "${case_id}-baseline" "$compiler" --phase "$root/meta/reflexive-normalize.gooo" \
		--input "$root/$source" --input-kind source --source "$root/$source" \
		--output-dir "$baseline_dir" --run-id "$case_id-baseline" --role baseline; then
		failed_count=$((failed_count + 1))
		continue
	fi
	if ! run_timed "${case_id}-candidate" "$compiler" --phase "$root/meta/reflexive-normalize.gooo" \
		--input "$baseline_dir/semantic-ir.json" --input-kind semantic-ir --source "$root/$source" \
		--output-dir "$candidate_dir" --run-id "$case_id-candidate" --role candidate; then
		failed_count=$((failed_count + 1))
		continue
	fi
	if ! run_timed "${case_id}-verify" "$verifier" --phase "$root/meta/reflexive-normalize.gooo" \
		--source "$root/$source" --baseline-dir "$baseline_dir" --candidate-dir "$candidate_dir" \
		--expected "$expected" --output "$case_dir/independent-verification.json"; then
		failed_count=$((failed_count + 1))
		continue
	fi
	actual=$(jq -r '.decision' "$case_dir/independent-verification.json")
	if [ "$actual" != "$expected" ]; then
		failed_count=$((failed_count + 1))
		continue
	fi
	case "$actual" in
		CLOSED) closed_count=$((closed_count + 1)) ;;
		UNKNOWN) unknown_count=$((unknown_count + 1)) ;;
		REFUTED) refuted_count=$((refuted_count + 1)) ;;
		*) failed_count=$((failed_count + 1)) ;;
	esac
done < <(jq -r '.cases[] | [.id,.source,.expected] | @tsv' "$root/contracts/denominator-v1.json")
compile_end=$(date +%s%3N)
compile_wall_ms=$((compile_end - compile_start))
if [ "$compile_wall_ms" -lt 1 ]; then compile_wall_ms=1; fi

output_files=$(find "$work/runs" -type f | wc -l | tr -d ' ')
output_bytes=$(find "$work/runs" -type f -print0 | xargs -0 stat -c '%s' | awk '{total += $1} END {print total + 0}')
conformance_start=${CONFORMANCE_START_MS:-$compile_start}
conformance_end=$(date +%s%3N)
conformance_wall_ms=$((conformance_end - conformance_start))
if [ "$conformance_wall_ms" -lt 1 ]; then conformance_wall_ms=1; fi

jq -n \
	--arg schema "gooo/reflexive-conformance/v1" \
	--argjson total "$case_count" --argjson executed "$((case_count - failed_count))" \
	--argjson reused 0 --argjson failed "$failed_count" --argjson unknown "$unknown_count" \
	--argjson closed "$closed_count" --argjson refuted "$refuted_count" \
	--argjson compile_wall_ms "$compile_wall_ms" --argjson conformance_wall_ms "$conformance_wall_ms" \
	--argjson peak_rss_bytes "$max_rss" --argjson output_files "$output_files" --argjson output_bytes "$output_bytes" \
	'{schema:$schema,decision:(if $failed == 0 and $closed == 1 and $unknown == 1 and $refuted == 1 then "CLOSED/UNKNOWN/REFUTED_CASES_CLOSED" else "FAIL_CLOSED" end),tests:{total:$total,executed:$executed,reused:$reused,failed:$failed,unknown:$unknown},cases:{closed:$closed,unknown:$unknown,refuted:$refuted},compile_wall_ms:$compile_wall_ms,conformance_wall_ms:$conformance_wall_ms,peak_rss_bytes:$peak_rss_bytes,output_files:$output_files,output_bytes:$output_bytes}' \
	> "$work/conformance-report.json"

jq -e '.tests.total == 3 and .tests.executed == 3 and .tests.reused == 0 and .tests.failed == 0 and .tests.unknown == 1 and .cases.closed == 1 and .cases.refuted == 1' "$work/conformance-report.json" > /dev/null
