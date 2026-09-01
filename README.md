# gooo-reflexive-compiler-slice

This repository demonstrates one bounded compiler phase, and only one:
`ONE_COMPILER_PHASE_ONLY`. The phase declared in
[`meta/reflexive-normalize.gooo`](meta/reflexive-normalize.gooo) owns the phase
graph. The current v2 graph supports the released three-activity topology and
the trial's four-activity split (`ParseSource` → `ValidateStableIDs` →
backend/replay), normalizes a Gooo source graph into canonical semantic IR, and
emits Go only as a derived backend artifact. The next execution reads the
first execution's generated semantic IR; it does not silently reread the
source.

Every execution also emits `terminal-record.json`, embeds the same
`terminal_record` in `receipt.json`, and carries the record into `generated.go`.
The record explains `CLOSED`, `UNKNOWN`, and `REFUTED` with the winning
stage/step/reason, the UNKNOWN six-field contract, a minimal cause-edge
frontier, and an evidence-derived counterexample digest.

The Go implementation interprets the operation programs declared by that
`.gooo` graph. The independent verifier in
[`cmd/gooo-reflexive-verify`](cmd/gooo-reflexive-verify) recomputes all artifact
digests, lineage, precedence, baseline/candidate behavior, and rollback
conditions from bytes on disk.

## Evidence boundary

The historical denominator remains exactly three cases: one `CLOSED`, one
`UNKNOWN`, and one `REFUTED`. The v0.3 terminal corpus adds a fixed two-normal,
five-UNKNOWN, two-REFUTED corpus and runs it through both the legacy
three-activity and split four-activity topologies. Reduction is fixed as
`REFUTED > UNKNOWN > CLOSED`. Every `UNKNOWN` record carries `stage`, `step`,
`reason`, `unknown_class`, `next_operation`, and `blocked_by`; missing fields
are a verification failure.

Only immutable releases listed in
[`contracts/upstream-lock-v1.json`](contracts/upstream-lock-v1.json) are used
as upstream inputs. The released `meta-ontology-go` CLI checks the phase graph
in CI, while the other two released producer assets are checksum-bound inputs.
The immutable evolution-trial release is separately locked in
[`contracts/evolution-trial-lock-v1.json`](contracts/evolution-trial-lock-v1.json)
and its candidate split is mechanically applied in CI. No mutable upstream
branch is consumed.

The project does not claim whole-language self-hosting, external utility, or a
global self-improvement result. Those remain `NOT_MADE` or `UNKNOWN` outside
this phase. The initial `main` bootstrap and post-bootstrap feature flow are
kept distinct in CI evidence; the root README is excluded from inventory
counts.

## CI and release

GitHub Actions is the verification authority. It runs Go 1.27, format, vet,
build, tests, immutable upstream checks, the three-case conformance run, and
the independent verifier, and the locked evolution-trial follow-up. It records
integer counts for Go/Gooo files and physical lines, regular files,
subdirectories, output files/bytes, generated artifact files/bytes, peak RSS,
compile, build, test, conformance, terminal-conformance, and integration wall
milliseconds, plus full/selected/executed/reused/failed/unknown test counts. The
follow-up closes the released `REFUTED` counterexample only for this phase,
with exact resolution pairs `1→2` valid topologies, `0→3` accepted trial
cases, and `1→2` localization stages under equal source, contract, and
toolchain digests.

The historical `v0.1.0` release is preserved as-is and is explicitly
non-durable (`immutable=false` in the GitHub release API); it is never reused
or modified. The manual, main-bound release workflow defaults to the new
`v0.3.0` version and requires the repository's immutable-releases setting to be
enabled through GitHub's official API before publication. It refuses an
existing tag or release, creates one annotated tag for the requested unused
version, publishes six exact assets with `SHA256SUMS`, and then fails closed
unless the REST release object reports `immutable=true`, the tag object and
target match, and every asset name, size, and digest matches the staged files.
A partial failure is left for inspection; tags and releases are never deleted,
overwritten, or reused.
