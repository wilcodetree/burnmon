# WS2 follow-up: Codex sessions that stay open never reach the live watcher

Owner: Wilco (request 2026-09-29). Executor: a Claude Code session in `C:\ZND\projects\burnmon`,
target `v0.4.0-alpha.8`. Never use em dashes.

## What was observed (2026-09-29, Codex in VS Code)

- Codex writes `C:\Users\WilcoDeTree\.codex\sessions\2026\09\29\rollout-2026-09-29T16-58-26-...jsonl`
  and keeps it open for the whole chat. It held 24 `token_count` lines in the format the adapter
  parses (`last_token_usage` present), the last one at 16:24:16Z.
- Windows still reported the file's LastWriteTime as 16:59:32 local at 18:2x local.
- BurnMon Dev showed no Codex at all (burn legend, Vendor strip TODAY 0).
- Setting the file's LastWriteTime by hand at 18:28:31 made "codex rollout- 27K" appear in the
  burn legend within seconds. So the adapter and store work; the change notice never arrives.

Cause (inferred from the above, matches the Win32 docs): `notifyMask` in
`internal\watch\watch_windows.go` asks only for `FILE_NAME | DIR_NAME | LAST_WRITE`. NTFS updates
last-write lazily for a file held open by its writer, so appends to an open file raise no
notification. Claude Code opens and closes per append, which is why it never showed this.

## 0. Every turn of a new-format rollout collapses into one event (found 2026-09-29, worst bug)

After the file was touched, Vendor strip Codex TODAY showed 27K: one turn, while the file holds
24 `token_count` lines from today. `codex.go` builds `RequestID` as
`sessionID + ":" + obj["ordinal"]`. Today's rollout lines have no top-level `ordinal` field
(sample: `{"timestamp":"2026-09-29T16:24:16.905Z","type":"event_msg","payload":{"type":"token_count",...}}`),
so every turn gets `":0"`, the same key, and the upsert keeps only the last one. The test
fixtures (`codex_test.go`, `testdata\codex\*.jsonl`) all still carry `ordinal`, so no test
caught it.

- Fall back, when `ordinal` is absent, to a key that is stable across incremental reads: the
  line's byte offset in the file (not a line counter, which restarts at the cursor).
- Add a fixture without `ordinal` (three turns) and a test that expects three events, also when
  read in two incremental passes.
- Count how many stored Codex events are affected: every event whose RequestID ends in ":0"
  from a file whose lines lack `ordinal`. Re-ingest those files from byte 0 after the fix.
  Report the Codex month total before and after.

## Wanted

1. Add `FILE_NOTIFY_CHANGE_SIZE` to `notifyMask`. Not enough on its own: the docs say size
   changes are also only seen when the cache flushes.
2. A tail poll as the real fix: every 2 s, for each native `.jsonl` file under the watched roots
   whose cursor moved in the last 60 minutes (or was created since start), open it, read its size
   from the handle (not from a directory listing), and call `IngestFile` when the size is past
   the stored cursor. Cap the set (for example 50 files) and log when the cap is hit.
3. The session label reads "rollout-", the first 8 characters of the file name, because the
   session has no project. Check whether this rollout's `session_meta` still carries `cwd`
   (VS Code extension, current Codex). If the field moved, read it from its new place. If there
   is truly no cwd, label it "codex" plus the last 4 characters of the session id, like the
   duplicate-label rule in `renderBurnLegend`.
4. Vendor strip TODAY for Codex must count these turns within its 60 s refresh.

5. Copilot in VS Code (added 2026-09-29): `copilotvsc.PollOnce` re-reads the whole OTel file
   from byte 0 every 5 s. Wilco's `copilot-otel.jsonl` was 46 MB at 18:36 and grows with every
   chat, so this cost only grows. Tail it with a stored offset like the JSONL adapters (reset to
   0 when the file shrinks or is replaced). Report the poll time before and after on the real
   file. Also check that the Vendor strip "Copilot (VS Code)" TODAY row counts these turns: at
   18:36 the burn chart and ticker showed copilot-vscode turns while the row still read 0.

## Proof

- Real run: a Codex chat in VS Code kept open for 15 minutes with at least 5 turns. Every turn
  shows in the burn chart within 5 s of its `token_count` line, without touching the file.
- Vendor strip Codex TODAY matches the sum of `last_token_usage` over today's lines (report both).
- Unit test for the tail poll with a file held open by the test while it appends.
- uicheck d0 to d22 pass. Version bump, README, STATUS, hub agent update brief.
