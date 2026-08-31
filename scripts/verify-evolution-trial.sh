#!/usr/bin/env bash
set -euo pipefail

root=${1:?repository root is required}
work=${2:?trial evidence work directory is required}
lock="$root/contracts/evolution-trial-lock-v1.json"
api="https://api.github.com"
token=${GH_TOKEN:-${GITHUB_TOKEN:-}}
auth_args=()
if [ -n "$token" ]; then
	auth_args=(-H "Authorization: Bearer $token")
fi

trial_repo=$(jq -r '.trial.repository' "$lock")
trial_tag=$(jq -r '.trial.tag' "$lock")
trial_root="$work/trial"
assets_dir="$trial_root/assets"
main_dir="$trial_root/main-artifact"
audit_dir="$trial_root/audit-artifact"
baseline_dir="$trial_root/baseline-source"
mkdir -p "$assets_dir" "$main_dir" "$audit_dir" "$baseline_dir/src"

api_get() {
	curl --silent --show-error --fail "${auth_args[@]}" \
		-H 'Accept: application/vnd.github+json' \
		-H 'X-GitHub-Api-Version: 2026-03-10' "$1"
}

release_json=$(api_get "$api/repos/$trial_repo/releases/tags/$trial_tag")
jq -e --slurpfile lock "$lock" '
	.id == ($lock[0].trial.release_id) and
	.tag_name == ($lock[0].trial.tag) and
	.immutable == true and .draft == false and .prerelease == false and
	([.assets[] | {id, name, size_bytes: .size, digest}] | sort_by(.name)) ==
	($lock[0].trial.assets | map({id, name, size_bytes, digest}) | sort_by(.name))
' <<<"$release_json" >/dev/null

tag_ref=$(api_get "$api/repos/$trial_repo/git/ref/tags/$trial_tag")
jq -e --slurpfile lock "$lock" '
	.ref == ("refs/tags/" + $lock[0].trial.tag) and
	.object.type == "tag" and .object.sha == $lock[0].trial.tag_object
' <<<"$tag_ref" >/dev/null
tag_body=$(api_get "$api/repos/$trial_repo/git/tags/$(jq -r '.object.sha' <<<"$tag_ref")")
jq -e --slurpfile lock "$lock" '
	.object.type == "commit" and .object.sha == $lock[0].trial.target_commit
' <<<"$tag_body" >/dev/null

while IFS=$'\t' read -r name digest url; do
	path="$assets_dir/$name"
	curl --silent --show-error --fail --location "${auth_args[@]}" -o "$path" "$url"
	echo "${digest#sha256:}  $path" | sha256sum -c -
done < <(jq -r '.trial.assets[] | [.name, .digest, .url] | @tsv' "$lock")
cmp "$assets_dir/experiment-dossier-v0.1.0.md" "$root/docs/evolution-trial-dossier-v0.1.0.md"

verify_artifact() {
	local artifact_key=$1
	local artifact_dir=$2
	local artifact_id
	artifact_id=$(jq -r --arg key "$artifact_key" '.trial[$key].id' "$lock")
	local artifact_json
	artifact_json=$(api_get "$api/repos/$trial_repo/actions/artifacts/$artifact_id")
	jq -e --arg key "$artifact_key" --slurpfile lock "$lock" '
		.id == ($lock[0].trial[$key].id) and
		.name == ($lock[0].trial[$key].name) and
		.size_in_bytes == ($lock[0].trial[$key].size_bytes) and
		.digest == ($lock[0].trial[$key].digest) and .expired == false
	' <<<"$artifact_json" >/dev/null
	curl --silent --show-error --fail --location "${auth_args[@]}" \
		-o "$artifact_dir/artifact.zip" "$api/repos/$trial_repo/actions/artifacts/$artifact_id/zip"
	unzip -q "$artifact_dir/artifact.zip" -d "$artifact_dir/unpacked"
}

verify_artifact main_artifact "$main_dir"
verify_artifact release_audit_artifact "$audit_dir"

main_run=$(api_get "$api/repos/$trial_repo/actions/runs/$(jq -r '.trial.main_run_id' "$lock")")
jq -e --slurpfile lock "$lock" '
	.id == $lock[0].trial.main_run_id and
	.head_sha == $lock[0].trial.target_commit and .conclusion == "success"
' <<<"$main_run" >/dev/null
main_job=$(api_get "$api/repos/$trial_repo/actions/jobs/$(jq -r '.trial.main_job_id' "$lock")")
jq -e --slurpfile lock "$lock" '
	.id == $lock[0].trial.main_job_id and .name == "verify-and-experiment" and .conclusion == "success"
' <<<"$main_job" >/dev/null
audit_run=$(api_get "$api/repos/$trial_repo/actions/runs/$(jq -r '.trial.release_audit_run_id' "$lock")")
jq -e --slurpfile lock "$lock" '
	.id == $lock[0].trial.release_audit_run_id and
	.head_sha == $lock[0].trial.target_commit and .conclusion == "success"
' <<<"$audit_run" >/dev/null
audit_job=$(api_get "$api/repos/$trial_repo/actions/jobs/$(jq -r '.trial.release_audit_job_id' "$lock")")
jq -e --slurpfile lock "$lock" '
	.id == $lock[0].trial.release_audit_job_id and .name == "release" and .conclusion == "success"
' <<<"$audit_job" >/dev/null

candidate_bundle=$(find "$main_dir/unpacked" -type f -path '*/delta-candidate/candidate-bundle.json' -print -quit)
test -n "$candidate_bundle"
candidate_bundle_digest=$(jq -r '.trial.candidate_bundle.digest' "$lock")
echo "${candidate_bundle_digest#sha256:}  $candidate_bundle" | sha256sum -c -
jq -e '.schema == "gooo/language-delta-forge/candidate-bundle/v1" and .decision == "CLOSED" and .counts == {added_cells:2,retired_cells:1,split_cells:1} and .semantic_graph_delta.exact_pair == true and .rollback_delta.exact_pair == true' "$candidate_bundle" >/dev/null
trial_candidate_phase=$(find "$main_dir/unpacked" -type f -name 'candidate-phase.gooo' -print -quit)
test -n "$trial_candidate_phase"
trial_candidate_phase_digest=$(jq -r '.trial.candidate_phase.digest' "$lock")
echo "${trial_candidate_phase_digest#sha256:}  $trial_candidate_phase" | sha256sum -c -

release_audit_json="$audit_dir/unpacked/release-audit.json"
test -f "$release_audit_json"
jq -e --slurpfile lock "$lock" '
	.schema == "gooo/evolution-trial/release-audit/v1" and
	.repository == $lock[0].trial.repository and .tag == $lock[0].trial.tag and
	.release_id == $lock[0].trial.release_id and .tag_object == $lock[0].trial.tag_object and
	.target_commit == $lock[0].trial.target_commit and .run_id == $lock[0].trial.release_audit_run_id and
	.job_id == $lock[0].trial.release_audit_job_id and .immutable == true and
	([.assets[] | {id,name,size,digest}] | sort_by(.name)) ==
	($lock[0].trial.assets | map({id,name,size:.size_bytes,digest}) | sort_by(.name))
' "$release_audit_json" >/dev/null

baseline_url=$(jq -r '.baseline_compiler.source_asset.url' "$lock")
baseline_name=$(jq -r '.baseline_compiler.source_asset.name' "$lock")
baseline_release=$(api_get "$api/repos/$(jq -r '.baseline_compiler.repository' "$lock")/releases/tags/$(jq -r '.baseline_compiler.tag' "$lock")")
jq -e --slurpfile lock "$lock" '
	.id == $lock[0].baseline_compiler.release_id and
	.immutable == true and .tag_name == $lock[0].baseline_compiler.tag and .draft == false and .prerelease == false
' <<<"$baseline_release" >/dev/null
baseline_tag_ref=$(api_get "$api/repos/$(jq -r '.baseline_compiler.repository' "$lock")/git/ref/tags/$(jq -r '.baseline_compiler.tag' "$lock")")
jq -e --slurpfile lock "$lock" '
	.object.type == "tag" and .object.sha == $lock[0].baseline_compiler.tag_object
' <<<"$baseline_tag_ref" >/dev/null
baseline_tag_body=$(api_get "$api/repos/$(jq -r '.baseline_compiler.repository' "$lock")/git/tags/$(jq -r '.object.sha' <<<"$baseline_tag_ref")")
jq -e --slurpfile lock "$lock" '
	.object.type == "commit" and .object.sha == $lock[0].baseline_compiler.target_commit
' <<<"$baseline_tag_body" >/dev/null
baseline_archive="$baseline_dir/$baseline_name"
curl --silent --show-error --fail --location "${auth_args[@]}" -o "$baseline_archive" "$baseline_url"
baseline_digest=$(jq -r '.baseline_compiler.source_asset.digest' "$lock")
echo "${baseline_digest#sha256:}  $baseline_archive" | sha256sum -c -
tar -xzf "$baseline_archive" -C "$baseline_dir/src"
baseline_root=$(find "$baseline_dir/src" -mindepth 1 -maxdepth 1 -type d -print -quit)
test -n "$baseline_root"

cat > "$trial_root/paths.env" <<EOF
CANDIDATE_BUNDLE=$candidate_bundle
BASELINE_ROOT=$baseline_root
BASELINE_PHASE=$baseline_root/meta/reflexive-normalize.gooo
EOF
