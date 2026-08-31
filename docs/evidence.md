# Evidence contract

This slice is an executable demonstration of a single Gooo-owned compiler
phase. It intentionally stops at the phase boundary.

## Authority and dataflow

`meta/reflexive-normalize.gooo` declares three executable activities. The
normalization activity declares the accepted input kind, declaration kinds,
stable-ID strategy, canonical ordering, and the CLOSED/UNKNOWN/REFUTED
handling. The backend activity explicitly marks generated Go as
`BACKEND_ONLY`, with the source graph as semantic authority.

The first run reads a `.gooo` source file and writes `semantic-ir.json`. The
candidate run receives that exact first IR path as its input and writes a
second IR. The independent verifier requires byte equality for both IR and
backend output, checks the source → IR → generated lineage, and confirms that
the baseline output remains available as a rollback target.

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

## Upstream boundary

`contracts/upstream-lock-v1.json` pins the release tag object, release target
commit, asset URL, and SHA-256 for each upstream input. CI checks the tag object
and downloads only those release assets. The meta-ontology-go release binary
checks the phase graph and its diagnostics are retained as evidence.

## Metrics

The CI metric file contains exact integer fields for `go_files`, `gooo_files`,
their physical lines, tracked files, subdirectories, generated output count and
bytes, peak RSS, compile/build/test/conformance wall milliseconds, and
`tests.total`, `tests.executed`, `tests.reused`, `tests.failed`, and
`tests.unknown`. The root `README.md` is explicitly excluded from the tracked
file inventory. The three fixed semantic cases are executed afresh, so reuse is
zero; the one UNKNOWN case is counted under `tests.unknown` and is not a test
failure.

The release workflow is fail-closed. If a previous publish attempt has already
created an annotated tag but no release, recovery verifies that the tag resolves
to the exact requested release SHA and publishes against that tag without
changing the tag. A new tag uses the current main SHA; an existing release,
wrong tag target, or checksum mismatch stops the workflow. No tag or release is
deleted or overwritten.

## Claims intentionally not made

The release reports `ONE_COMPILER_PHASE_ONLY`. It does not claim that Gooo
compiles itself end-to-end, that the phase has external utility, or that the
whole language is globally self-hosting. Those claims remain
`UNKNOWN`/`NOT_MADE` until separate released-input evidence exists.
