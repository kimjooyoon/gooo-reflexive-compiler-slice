#!/usr/bin/env bash
set -euo pipefail

root=${1:?repository root is required}
work=${2:?work directory is required}
mkdir -p "$work/upstream"
records="$work/upstream/records.ndjson"
: > "$records"

while IFS=$'\t' read -r producer release target tag_object asset url expected role; do
	if [ -z "$producer" ]; then
		continue
	fi
	repository=${producer#github.com/}
	tag=$(git ls-remote "https://github.com/${repository}.git" "refs/tags/${release}" | awk 'NR == 1 { print $1 }')
	if [ "$tag" != "$tag_object" ]; then
		echo "immutable tag object mismatch for $producer $release" >&2
		exit 1
	fi
	destination="$work/upstream/$asset"
	curl --fail --silent --show-error --location --output "$destination" "$url"
	actual=$(sha256sum "$destination" | awk '{print $1}')
	if [ "$actual" != "$expected" ]; then
		echo "immutable asset digest mismatch for $asset" >&2
		exit 1
	fi
	jq -cn --arg producer "$producer" --arg release "$release" --arg target "$target" \
		--arg tag_object "$tag_object" --arg asset "$asset" --arg digest "sha256:$actual" --arg role "$role" \
		'{producer:$producer,release:$release,target_commit:$target,tag_object:$tag_object,asset:$asset,digest:$digest,role:$role}' >> "$records"
done < <(jq -r '.inputs[] | [.producer,.release,.target_commit,.tag_object,.asset,.url,.sha256,.role] | @tsv' "$root/contracts/upstream-lock-v1.json")

mkdir -p "$work/upstream/meta"
tar -xzf "$work/upstream/gooo-linux-amd64.tar.gz" -C "$work/upstream/meta"
"$work/upstream/meta/gooo" check "$root/meta/reflexive-normalize.gooo" --json > "$work/upstream/released-phase-check.json"
jq -e '.status == "ok" and (.diagnostics | length == 0)' "$work/upstream/released-phase-check.json" > /dev/null

jq -s --arg schema "gooo/reflexive-upstream-release-verification/v1" \
	--arg phase_check_digest "$(sha256sum "$work/upstream/released-phase-check.json" | awk '{print "sha256:" $1}')" \
	'{schema:$schema,inputs:.,released_phase_check:{status:"ok",digest:$phase_check_digest}}' \
	"$records" > "$work/upstream/report.json"
