# claudecost, session log

One paragraph per work session, newest on top.

## 2026-09-22, v0.1 Step 1: schema, store, Claude adapter

Landed the schema/store/adapter split from `C:\ZND\projects\burnmon\02_roadmap\2026-09-22_v0.1_spec.md`
Step 1: `internal\schema.Event`, a SQLite-backed `internal\store` (events, cursors, meta,
`modernc.org/sqlite`, no cgo), the `internal\adapter.Adapter` interface, and
`internal\adapter\claude` (the Claude/Cowork parser moved out of `internal\scan\parse.go`
and rewritten for incremental byte-offset reads). `internal\dataset` was rewired around
the store end to end, and the old gob parse cache is gone. Two real bugs surfaced during
real-data verification against Wilco's own transcripts and both are fixed: the store's
original dedup key, `(vendor, request_id)`, was global and let Cowork's mirrored
session files steal each other's events (one real session collapsed from 67 calls to 2);
it is now scoped to `(vendor, session_id, request_id)`. A final whole-branch review then
caught a second, more serious one before it could ship: the incremental reader was
consuming a trailing partial line and permanently losing its turns once a live-growing
file's line finished writing later, exactly the code path the store exists for and the
one path `-no-cache`-based verification could never exercise; the reader is now
`bufio.Reader`-based and proven by a dedicated offset-accounting test
(`TestParseDoesNotConsumeTrailingPartialLine`). One intentional, sign-off'd diff from
v0.0.1: the surface vocabulary changed from `cowork`/`code`/`code_agent`/`chat` to
`cli`/`code_agent`/`desktop`/`unknown` (cowork and chat merged into desktop). Final
verification: `burnmon-cli.exe report` from the store against a v0.0.1 baseline binary
(commit `3d00f22`), both run `-no-cache` against the same real two months of transcripts,
came back byte-identical apart from that one acknowledged surface field. `go test ./...`
and `.\build.ps1` are green. Four findings were parked as deferred minors for Step 2/3
(a chunk-relative synthetic dedup key fallback, a strict-vs-`>=` upsert guard mismatch,
`Session.Surface` sourced from `events[0]` instead of first-non-empty, and the
`<synthetic>`-model filter moving from after-dedup to before-dedup, likely an
improvement but an unremarked semantics change). Tag command below, not run.

```
git tag -a v0.1.0-alpha.1 -m "Step 1: schema, store, Claude adapter"
```
