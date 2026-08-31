# gooo-reflexive-compiler-slice

This repository demonstrates one bounded compiler phase, and only one:
`ONE_COMPILER_PHASE_ONLY`. The phase declared in
[`meta/reflexive-normalize.gooo`](meta/reflexive-normalize.gooo) reads a Gooo
source graph, normalizes its semantic declarations into a canonical semantic
IR, and emits Go only as a derived backend artifact. The next execution reads
the first execution's generated semantic IR; it does not silently reread the
source.

The Go implementation interprets the operation programs declared by that
`.gooo` graph. The independent verifier in
[`cmd/gooo-reflexive-verify`](cmd/gooo-reflexive-verify) recomputes all artifact
digests, lineage, precedence, baseline/candidate behavior, and rollback
conditions from bytes on disk.

## Evidence boundary

The fixed denominator is exactly three cases: one `CLOSED`, one `UNKNOWN`, and
one `REFUTED`. Reduction is fixed as `REFUTED > UNKNOWN > CLOSED`. Every
`UNKNOWN` record carries `stage`, `step`, `reason`, `unknown_class`,
`next_operation`, and `blocked_by`; missing fields are a verification failure.

Only immutable releases listed in
[`contracts/upstream-lock-v1.json`](contracts/upstream-lock-v1.json) are used
as upstream inputs. The released `meta-ontology-go` CLI checks the phase graph
in CI, while the other two released producer assets are checksum-bound inputs.
No mutable upstream branch is consumed.

The project does not claim whole-language self-hosting, external utility, or a
global self-improvement result. Those remain `NOT_MADE` or `UNKNOWN` outside
this phase. The initial `main` bootstrap and post-bootstrap feature flow are
kept distinct in CI evidence; the root README is excluded from inventory
counts.

## CI and release

GitHub Actions is the verification authority. It runs Go 1.27, format, vet,
build, tests, immutable upstream checks, the three-case conformance run, and
the independent verifier. It records integer counts for Go/Gooo files and
physical lines, files, subdirectories, output files/bytes, peak RSS, compile,
build, test, and conformance wall milliseconds, plus test totals/executed,
reused, failed, and unknown.

The historical `v0.1.0` release is preserved as-is and is explicitly
non-durable (`immutable=false` in the GitHub release API); it is never reused
or modified. The manual, main-bound release workflow defaults to the new
`v0.1.1` version and requires the repository's immutable-releases setting to be
enabled through GitHub's official API before publication. It refuses an
existing tag or release, creates one annotated tag for the requested unused
version, publishes six exact assets with `SHA256SUMS`, and then fails closed
unless the REST release object reports `immutable=true`, the tag object and
target match, and every asset name, size, and digest matches the staged files.
A partial failure is left for inspection; tags and releases are never deleted,
overwritten, or reused.
