# Gooo evolution trial dossier

This is the committed experiment boundary and interpretation. The run-specific
machine report and generated dossier are uploaded by GitHub Actions.

The single experiment observes the real `NormalizeSource` phase in the
immutable `gooo-reflexive-compiler-slice v0.1.1` release. That released phase
parses declarations, binds stable IDs, sorts the semantic IR by stable ID,
refutes duplicate stable IDs, and records an UNKNOWN when its required entity
is missing.

The immutable `gooo-language-delta-forge v0.1.2` receives that observation and
emits one exact split candidate: retire the coarse `NormalizeSource` cell and
add two cells, `ParseSource` and `ValidateStableIDs`. The candidate carries an
exact inverse rollback and an exact integer resolution pair of 1 before and 2
after phase-localization cells, but that pair is usable only when the run binds
the same source and toolchain digests.

The candidate `.gooo` phase is generated from the released candidate bundle by
the caller-owned adapter. Candidate Go is never hand-authored as semantic
authority. The candidate phase intentionally exposes the released compiler's
composition boundary: it has four executable activities, while the released
compiler phase parser accepts exactly three. If the released compiler rejects
the candidate, the experiment records `REFUTED`, preserves the baseline, and
does not invent candidate IR or backend artifacts.

The immutable `gooo-causal-verification-runner v0.1.1` selects the affected
candidate corpus test, reuses one exact immutable proof for a stable control,
and compares both with the independent full oracle. A selective result never
suppresses a full-oracle counterexample.

Acceptance requires candidate artifacts, equal replay digests, preservation of
the released one-`CLOSED`/one-`UNKNOWN`/one-`REFUTED` corpus, causal/full-oracle
agreement, a possible rollback, and the matched integer pair. Whole-language
self-improvement and external utility are not established by this bounded
phase experiment and remain `UNKNOWN`/`NOT_MADE`.
