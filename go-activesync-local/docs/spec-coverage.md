# Spec-coverage matrix

This file is a human-readable companion to
`[internal/spec/coverage.csv](../internal/spec/coverage.csv)` — the source of
truth that the `speclint` tool consumes in CI.

Every row in the CSV is one atomic spec requirement with a stable `spec_id`.
Every test that exercises a requirement carries a `// SPEC: <spec_id>` marker;
the linter cross-checks the two and fails the build if any `required`
requirement has no test, or any marker references an id absent from the
matrix.

## Column reference


| column        | description                                                    |
| ------------- | -------------------------------------------------------------- |
| `spec_id`     | stable identifier, conventionally `<DOC>/<section>` or similar |
| `doc`         | source document, e.g. `MS-ASWBXML`, `OMA-WBXML-1.3`            |
| `section`     | section anchor, e.g. `§2.1.2.1.1`                              |
| `requirement` | one-line summary of the atomic requirement                     |
| `status`      | `required`, `optional`, or `out_of_scope`                      |


## Documents tracked

- OMA-WBXML-1.3 — base WBXML format (global tokens, `mb_u_int32`, header).
- MS-ASWBXML — EAS code page tables (0..24).
- MS-ASHTTP — HTTP transport (request line, query encodings, headers, status).
- MS-ASCMD — command semantics, ABNF, status codes.
- MS-ASEMAIL / MS-ASCAL / MS-ASCNTC / MS-ASTASK — PIM domain classes (14.1).
- MS-ASPROV — provisioning policy.
- MS-OXDISCO / MS-ASAB — POX Autodiscover.

## Bootstrap row

The matrix ships with a single self-referential `SPEC-LINT/self-test` row that
locks in the linter contract: the linter's own unit tests carry the marker, so
the linter is always its own first verified consumer.