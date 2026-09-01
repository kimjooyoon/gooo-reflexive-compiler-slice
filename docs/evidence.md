# Evidence contract

This slice is an executable demonstration of a single Gooo-owned compiler
phase. It intentionally stops at the phase boundary.

## Authority and dataflow

`meta/reflexive-normalize.gooo` owns the executable topology. The legacy
three-activity graph remains supported, while the adopted v2 graph declares
four typed activities: `ParseSource`, `ValidateStableIDs`, `EmitBackend`, and
`VerifyReplay`. It declares the accepted input kind, declaration kinds,
stable-ID strategy, canonical ordering, split migration, typed edges, and the
CLOSED/UNKNOWN/REFUTED handling. The backend activity explicitly marks
generated Go as `BACKEND_ONLY`, with the source graph as semantic authority.

The first run reads a `.gooo` source file and writes `semantic-ir.json`. The
candidate run receives that exact first IR path as its input and writes a
second IR. Both runs also write `terminal-record.json`; the record is copied
into `receipt.json` and `generated.go`. The independent verifier requires byte
equality for both IR, backend, and terminal output, checks the source → IR →
generated lineage, validates the minimal cause-edge frontier and counterexample
digest, and confirms that the baseline output remains available as a rollback
target.

## Fixed denominator

`contracts/denominator-v1.json` is the only case inventory and contains exactly
three cases:

| Case | Expected state | Counterexample or evidence |
| --- | --- | --- |
| `CLOSED_CANONICAL_INPUT` | `CLOSED` | required entity and unique stable IDs |
| `UNKNOWN_MISSING_REQUIRED_ENTITY` | `UNKNOWN` | required entity is absent |
| `REFUTED_DUPLICATE_STABLE_ID` | `REFUTED` | two declarations share one stable ID |

The phase reduces states in the fixed order `REFUTED > UNKNOWN > CLOSED`.
An UNKNOWN is not treated as success and cannot be promoted by a human note.

The v0.3 corpus in `contracts/terminal-corpus-v1.json` contains two CLOSED,
five distinct UNKNOWN classes, and two REFUTED cases. Each is run against both
the three-activity legacy phase and the four-activity split phase, for 18
selected executions. The original three-case denominator remains the locked
evolution-trial control.

## Upstream boundary

`contracts/upstream-lock-v1.json` pins the release tag object, release target
commit, asset URL, and SHA-256 for each upstream input. CI checks the tag object
and downloads only those release assets. The meta-ontology-go release binary
checks the phase graph and its diagnostics are retained as evidence. CI also
downloads the immutable evolution-trial release, verifies all six release
assets and both Actions artifacts, and mechanically applies its split bundle
into a candidate phase. The generated candidate must byte-match the committed
`.gooo` graph.

## Metrics

The CI metric file contains exact integer fields for `go_files`, `gooo_files`,
their physical lines, regular files, subdirectories, generated artifact count
and bytes, peak RSS, compile/build/test/conformance/terminal-conformance/
integration wall milliseconds, and `tests.total`, `tests.selected`,
`tests.executed`, `tests.reused`, `tests.failed`, and `tests.unknown`. The root
`README.md` is explicitly excluded from the regular-file inventory. The
terminal corpus is executed afresh, so reuse is zero; its ten UNKNOWN topology
runs are evidence, not test failures.

The historical `v0.1.0` release is retained without mutation and is recorded as
non-durable because its GitHub release API object reports `immutable=false`.
The corrective release workflow uses an unused version (default `v0.3.0`),
refuses any existing tag or release, and never repairs a failed tag by deleting
or rewriting it. After the repository immutable-releases setting is enabled
through the official REST API, the workflow creates one annotated tag for the
exact main SHA, publishes six assets, and verifies through REST that the release
is non-draft, non-prerelease, `immutable=true`, and has the exact six asset
names, sizes, and digests staged by CI. It also verifies the annotated tag
object and commit target. Any mismatch stops the workflow.

## Claims intentionally not made

The release reports `ONE_COMPILER_PHASE_ONLY`. It does not claim that Gooo
compiles itself end-to-end, that the phase has external utility, or that the
whole language is globally self-hosting. Those claims remain
`UNKNOWN`/`NOT_MADE` until separate released-input evidence exists.
