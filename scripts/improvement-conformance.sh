#!/usr/bin/env bash
set -Eeuo pipefail
trap 'echo "improvement conformance failed at ${BASH_SOURCE[0]}:${LINENO}: ${BASH_COMMAND}" >&2' ERR

root=${1:?repository root is required}
work=${2:?improvement work directory is required}
compiler=${3:?current compiler executable is required}
verifier=${4:?current verifier executable is required}

started=$(date +%s%3N)
mkdir -p "$work/improvement/timing" "$work/improvement/before" "$work/improvement/after"
bash "$root/scripts/verify-evolution-trial.sh" "$root" "$work"
source "$work/trial/paths.env"

baseline_compiler="$work/improvement/before-compiler"
baseline_verifier="$work/improvement/before-verifier"
(cd "$BASELINE_ROOT" && go build -trimpath -o "$baseline_compiler" ./cmd/gooo-reflexive-compiler-slice && go build -trimpath -o "$baseline_verifier" ./cmd/gooo-reflexive-verify)

candidate_phase="$work/improvement/candidate-phase.gooo"
bash "$root/scripts/apply-split-candidate.sh" "$BASELINE_PHASE" "$CANDIDATE_BUNDLE" "$candidate_phase"
cmp "$candidate_phase" "$root/meta/reflexive-normalize.gooo"

contract="$BASELINE_ROOT/contracts/denominator-v1.json"
contract_digest=$(sha256sum "$contract" | awk '{print "sha256:" $1}')
test "$contract_digest" = "$(sha256sum "$root/contracts/denominator-v1.json" | awk '{print "sha256:" $1}')"
toolchain_digest=$(go version | sha256sum | awk '{print "sha256:" $1}')
source_tree_digest=$(find "$BASELINE_ROOT/examples/cases" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | awk '{print "sha256:" $1}')
candidate_phase_digest=$(sha256sum "$candidate_phase" | awk '{print "sha256:" $1}')
baseline_phase_digest=$(sha256sum "$BASELINE_PHASE" | awk '{print "sha256:" $1}')

before_summary="$work/improvement/before-phase-summary.json"
after_summary="$work/improvement/after-phase-summary.json"
"$compiler" --inspect-phase --json --phase "$BASELINE_PHASE" > "$before_summary"
"$compiler" --inspect-phase --json --phase "$candidate_phase" > "$after_summary"
jq -e '.valid == true and .activity_count == 3 and .localization_stages == 1' "$before_summary" >/dev/null
jq -e '.valid == true and .activity_count == 4 and .localization_stages == 2' "$after_summary" >/dev/null

max_rss_kib=0
run_timed() {
	local label=$1
	shift
	local timing="$work/improvement/timing/$label.txt"
	set +e
	/usr/bin/time -v -o "$timing" "$@"
	local status=$?
	set -e
	local rss
	rss=$(awk -F: '/Maximum resident set size/ {gsub(/^[ \t]+/, "", $2); print $2 + 0}' "$timing")
	if [ "${rss:-0}" -gt "$max_rss_kib" ]; then
		max_rss_kib=$rss
	fi
	return "$status"
}

case_records="$work/improvement/cases.ndjson"
: > "$case_records"
before_accepted=0
after_accepted=0
before_closed=0
before_unknown=0
before_refuted=0
after_closed=0
after_unknown=0
after_refuted=0

while IFS=$'\t' read -r case_id source expected; do
	[ -n "$case_id" ] || continue
	source_path="$BASELINE_ROOT/$source"
	input_digest=$(sha256sum "$source_path" | awk '{print "sha256:" $1}')
	case_dir="$work/improvement"
	before_base="$case_dir/before/$case_id/baseline"
	before_candidate="$case_dir/before/$case_id/candidate"
	after_base="$case_dir/after/$case_id/baseline"
	after_candidate="$case_dir/after/$case_id/candidate"
	mkdir -p "$before_base" "$before_candidate" "$after_base" "$after_candidate"

	before_baseline_status=0
	if run_timed "before-$case_id-baseline" "$baseline_compiler" --phase "$BASELINE_PHASE" \
		--input "$source_path" --input-kind source --source "$source_path" \
		--output-dir "$before_base" --run-id "$case_id-before-baseline" --role baseline; then
		:
	else
		before_baseline_status=$?
	fi
	before_candidate_status=0
	if [ "$before_baseline_status" -eq 0 ] && [ -f "$before_base/semantic-ir.json" ]; then
		if run_timed "before-$case_id-candidate" "$baseline_compiler" --phase "$candidate_phase" \
			--input "$before_base/semantic-ir.json" --input-kind semantic-ir --source "$source_path" \
			--output-dir "$before_candidate" --run-id "$case_id-before-candidate" --role candidate; then
			:
		else
			before_candidate_status=$?
		fi
	else
		before_candidate_status=1
	fi
	before_decision="REFUTED"
	if [ -f "$before_base/receipt.json" ]; then
		before_decision=$(jq -r '.decision' "$before_base/receipt.json")
	fi
	if [ "$before_candidate_status" -eq 0 ] && [ -f "$before_candidate/generated.go" ]; then
		before_accepted=$((before_accepted + 1))
	fi

	after_baseline_status=0
	if run_timed "after-$case_id-baseline" "$compiler" --phase "$candidate_phase" \
		--input "$source_path" --input-kind source --source "$source_path" \
		--output-dir "$after_base" --run-id "$case_id-after-baseline" --role baseline; then
		:
	else
		after_baseline_status=$?
	fi
	after_candidate_status=0
	if [ "$after_baseline_status" -eq 0 ] && [ -f "$after_base/semantic-ir.json" ]; then
		if run_timed "after-$case_id-candidate" "$compiler" --phase "$candidate_phase" \
			--input "$after_base/semantic-ir.json" --input-kind semantic-ir --source "$source_path" \
			--output-dir "$after_candidate" --run-id "$case_id-after-candidate" --role candidate; then
			:
		else
			after_candidate_status=$?
		fi
	else
		after_candidate_status=1
	fi
	after_verify_status=0
	if [ "$after_baseline_status" -eq 0 ] && [ "$after_candidate_status" -eq 0 ]; then
		if run_timed "after-$case_id-verify" "$verifier" --phase "$candidate_phase" \
			--source "$source_path" --baseline-dir "$after_base" --candidate-dir "$after_candidate" \
			--expected "$expected" --output "$case_dir/after/$case_id/independent-verification.json"; then
			:
		else
			after_verify_status=$?
		fi
	else
		after_verify_status=1
	fi
	after_decision="REFUTED"
	if [ -f "$after_candidate/receipt.json" ]; then
		after_decision=$(jq -r '.decision' "$after_candidate/receipt.json")
	fi
	if [ "$after_candidate_status" -eq 0 ] && [ "$after_verify_status" -eq 0 ] && [ -f "$after_candidate/generated.go" ]; then
		after_accepted=$((after_accepted + 1))
	fi
	case "$before_decision" in CLOSED) before_closed=$((before_closed + 1));; UNKNOWN) before_unknown=$((before_unknown + 1));; REFUTED) before_refuted=$((before_refuted + 1));; esac
	case "$after_decision" in CLOSED) after_closed=$((after_closed + 1));; UNKNOWN) after_unknown=$((after_unknown + 1));; REFUTED) after_refuted=$((after_refuted + 1));; esac

	before_execution_digest=""
	after_execution_digest=""
	if [ -f "$before_candidate/receipt.json" ]; then before_execution_digest=$(jq -r '.execution_digest' "$before_candidate/receipt.json"); fi
	if [ -f "$after_candidate/receipt.json" ]; then after_execution_digest=$(jq -r '.execution_digest' "$after_candidate/receipt.json"); fi
	jq -n --arg id "$case_id" --arg source "$source" --arg expected "$expected" --arg input "$input_digest" \
		--arg before "$before_decision" --arg after "$after_decision" --arg before_digest "$before_execution_digest" --arg after_digest "$after_execution_digest" \
		--argjson before_status "$before_candidate_status" --argjson after_status "$after_candidate_status" --argjson verify_status "$after_verify_status" \
		'{id:$id,source:$source,expected:$expected,input_digest:$input,before:{decision:$before,accepted:($before_status == 0),execution_digest:$before_digest},after:{decision:$after,accepted:($after_status == 0 and $verify_status == 0),execution_digest:$after_digest}}' \
		>> "$case_records"
done < <(jq -r '.cases[] | [.id,.source,.expected] | @tsv' "$contract")

ended=$(date +%s%3N)
integration_wall_ms=$((ended - started))
if [ "$integration_wall_ms" -lt 1 ]; then integration_wall_ms=1; fi
cases_json=$(jq -s . "$case_records")
before_summary_json=$(cat "$before_summary")
after_summary_json=$(cat "$after_summary")

jq -n --arg schema "gooo/reflexive-improvement/v1" --arg status "CLOSED" \
	--arg baseline_phase_digest "$baseline_phase_digest" --arg candidate_phase_digest "$candidate_phase_digest" \
	--arg candidate_bundle_digest "$(jq -r '.trial.candidate_bundle.digest' "$root/contracts/evolution-trial-lock-v1.json")" \
	--arg trial_candidate_phase_digest "$(jq -r '.trial.candidate_phase.digest' "$root/contracts/evolution-trial-lock-v1.json")" \
	--arg candidate_digest "$(jq -r '.candidate_digest' "$CANDIDATE_BUNDLE")" \
	--arg delta_digest "$(jq -r '.delta_digest' "$CANDIDATE_BUNDLE")" \
	--arg source_tree_digest "$source_tree_digest" --arg contract_digest "$contract_digest" --arg toolchain_digest "$toolchain_digest" \
	--argjson before_topology "$before_summary_json" --argjson after_topology "$after_summary_json" \
	--argjson cases "$cases_json" --argjson before_accepted "$before_accepted" --argjson after_accepted "$after_accepted" \
	--argjson before_closed "$before_closed" --argjson before_unknown "$before_unknown" --argjson before_refuted "$before_refuted" \
	--argjson after_closed "$after_closed" --argjson after_unknown "$after_unknown" --argjson after_refuted "$after_refuted" \
	--argjson peak_rss_kib "$max_rss_kib" --argjson integration_wall_ms "$integration_wall_ms" \
	--slurpfile lock "$root/contracts/evolution-trial-lock-v1.json" \
	'{schema:$schema,status:$status,trial:{repository:$lock[0].trial.repository,release_id:$lock[0].trial.release_id,tag:$lock[0].trial.tag,immutable:$lock[0].trial.immutable,tag_object:$lock[0].trial.tag_object,target_commit:$lock[0].trial.target_commit,main_run_id:$lock[0].trial.main_run_id,main_job_id:$lock[0].trial.main_job_id,main_artifact:$lock[0].trial.main_artifact,release_audit_run_id:$lock[0].trial.release_audit_run_id,release_audit_job_id:$lock[0].trial.release_audit_job_id,release_audit_artifact:$lock[0].trial.release_audit_artifact,candidate_phase_digest:$trial_candidate_phase_digest,counterexample:$lock[0].trial.counterexample},candidate:{bundle_digest:$candidate_bundle_digest,candidate_digest:$candidate_digest,delta_digest:$delta_digest,applied_phase_digest:$candidate_phase_digest,mechanically_matches_root:true},topology:{before:$before_topology,after:$after_topology},cases:$cases,distributions:{before:{CLOSED:$before_closed,UNKNOWN:$before_unknown,REFUTED:$before_refuted},after:{CLOSED:$after_closed,UNKNOWN:$after_unknown,REFUTED:$after_refuted}},resolution_pairs:{supported_valid_topology_cardinalities:{before:$before_topology.localization_stages,after:$after_topology.localization_stages,unit:"valid-topology-cardinalities"},accepted_trial_candidate_cases:{before:$before_accepted,after:$after_accepted,unit:"cases"},coarse_localization_stages:{before:$before_topology.localization_stages,after:$after_topology.localization_stages,unit:"phase-localization-stages"}},same_digest_conditions:{source_tree_digest:$source_tree_digest,contract_digest:$contract_digest,toolchain_digest:$toolchain_digest},closure_receipt:{schema:"gooo/reflexive-improvement-closure/v1",state:"CLOSED",stage:"IMPROVEMENT",step:"RESOLVE_TRIAL_COUNTEREXAMPLE",reason:"GRAPH_SEMANTICS_ACCEPT_SPLIT_CANDIDATE",unknown_class:"",next_operation:"RETAIN_BASELINE_AND_CANDIDATE_EVIDENCE",blocked_by:[],trial_refutation_state:"REFUTED",trial_refutation_error:$lock[0].trial.counterexample.error},integration:{wall_ms:$integration_wall_ms,peak_rss_kib:$peak_rss_kib},authority:{verification_authority:"GITHUB_ACTIONS",repository_writes:0,upstream_writes:0,local_test_executions:0,protected_core_adoption:0}}' \
	> "$work/improvement-report.json"

jq -e --argjson cases "$cases_json" --argjson before "$before_accepted" --argjson after "$after_accepted" '
	.status == "CLOSED" and
	.resolution_pairs.supported_valid_topology_cardinalities.before == 1 and
	.resolution_pairs.supported_valid_topology_cardinalities.after == 2 and
	.resolution_pairs.accepted_trial_candidate_cases.before == $before and
	.resolution_pairs.accepted_trial_candidate_cases.after == $after and
	$before == 0 and $after == 3 and
	.distributions.before == {CLOSED:1,UNKNOWN:1,REFUTED:1} and
	.distributions.after == {CLOSED:1,UNKNOWN:1,REFUTED:1} and
	([ $cases[] | select(.expected == .after.decision and .after.accepted == true) ] | length) == 3 and
		(.cases | length == 3) and
	.closure_receipt.state == "CLOSED" and .closure_receipt.trial_refutation_state == "REFUTED"
' "$work/improvement-report.json" >/dev/null
