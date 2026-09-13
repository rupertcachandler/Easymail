# AGENTS.md

Operational rules for AI agents (and humans following the same workflow)
contributing to this repository. Keep these requirements satisfied at all
times; do not merge or hand off work that violates them.

## 1. Always validate locally with `make ci`

Before declaring a task complete, before opening a PR, and before pushing
any branch, you **must** run:

```sh
make ci
```

`make ci` mirrors the exact steps executed by `.github/workflows/ci.yml`:

1. `go mod verify`
2. `go vet ./...`
3. `golangci-lint run ./...` (auto-installs if absent)
4. `go test -race -count=1 -covermode=atomic -coverprofile=cover.out ./...`
5. `go run ./internal/spec/cmd/covergate cover.out` — per-package coverage
   thresholds (`wbxml/`, `eas/` ≥ 90 %; `client/`, `autodiscover/` ≥ 80 %).
6. `go run ./internal/spec/cmd/speclint` — every `// SPEC:` marker maps to
   an entry in `internal/spec/coverage.csv`.
7. `go test ./wbxml -run='^$' -fuzz=FuzzDecode -fuzztime=30s` — short fuzz
   smoke run.

If any step fails locally, fix it before continuing — that step *will* fail
in CI as well.

When the workflow file changes, update `make ci` in the same commit so the
two stay in lockstep.

## 2. Spec-first, test-first

Every protocol-relevant requirement in this repository is anchored in a
Microsoft Open Specifications document (MS-ASWBXML, MS-ASCMD, MS-ASHTTP,
MS-ASEMAIL, …). Workflow:

1. Add the requirement to `internal/spec/coverage.csv` with a stable
   `spec_id`.
2. Write a failing test that references the requirement via a
   `// SPEC: <spec_id>` comment.
3. Implement the minimum code that makes the test pass.

`speclint` will reject orphan `// SPEC:` markers and missing requirements.

## 3. Per-package coverage thresholds are enforced

`covergate` blocks merges if any of the gated packages drops below its
threshold. When you reduce coverage in a package — typically by adding a
new branch — add tests for the new branch in the same change.

## 4. Linter configuration is the source of truth

`.golangci.yml` defines the active linter set. Do not bypass it with
`//nolint` directives unless there is a documented reason in the comment.
If a check is consistently noisy, propose a config change rather than
sprinkling suppressions.

## 5. Module path and authorship

- Module path: `github.com/remdev/go-activesync`. Do not introduce other
  paths in imports, examples, or docs.
- Append new exported fields at the **end** of configuration structs (for
  example `client.Config`) so positional composite literals in downstream code
  remain compatible across minor updates.
- Commit author / committer is governed by the local git config. Do not
  rewrite history that has already been pushed to `origin`.

## 6. No editing of `internal/spec/coverage.csv` rows post-implementation

Once a requirement has been implemented and pushed, treat its row as
append-only. To deprecate a requirement, add a follow-up row marking it
superseded; do not silently rename or delete the original `spec_id`.

## 7. Commits

- One logical change per commit; concise, lower-case message focused on
  the *why*.
- Do not commit until `make ci` is green.
- Do not commit `cover.out`, `cover-*.out`, or other build artefacts;
  `.gitignore` already excludes them.
