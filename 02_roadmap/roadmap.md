# burnmon, roadmap

Priority order lives here and only here.

1. v0.1 (2026-10-17): schema and SQLite store, Codex adapter, the live Now page.
   Spec: `2026-09-22_v0.1_spec.md`. Session prompt: `2026-09-22_v0.1_session_prompt.md`.
   Steps 1 to 3 landed 2026-09-22 (`693ea5a`). Fixes from the first live run:
   `2026-09-22_v0.1.1_fix_spec.md` (memory, Codex missing, running chart, labels, windows, copy).
2. v0.1.2 patch (week 39): Codex sessions appear without Refresh, smooth running chart,
   version strings and STATUS. Spec: `2026-09-22_v0.1.2_patch_spec.md`.
3. v0.2 (2026-11-14), one release, two sessions a week: migrations and tool_calls, five
   tabs (Now default, History with filters, Sessions, Tools, About), vendor strip, English
   only, owner split, insight package (re-prefill, compaction, runway, spike drawer),
   forecast behind a visible gate, Hermes and Copilot CLI adapters, Copilot VS Code OTel
   check. Spec: `2026-09-22_v0.2_spec.md`. Session prompts, one per session, with the
   checklist: `2026-09-22_v0.2_session_prompts.md`. Grilled 2026-09-22. **DONE 2026-09-23,
   tagged `v0.2.0`, all sixteen spec items shipped, release-candidate Done-when and VERIFY
   pass clean (46A), no fails carried into the tag.**
4. v0.2.1 patch: Refresh went "Not Responding" (98.5% CPU, 1,438 MB, 175 MB/s disk).
   Measured root cause: the Sessions tab fired one bmSessionInsight call per kept session
   unconditionally on every page load (real store: ~1,700 sessions), all serialised through
   the store's one connection and blocking the WebView2 UI thread call by call; Collect
   itself was fast (1.2-2.6s) and not the driver. Spec:
   `2026-09-23_v0.2.1_hang_patch.md`. **DONE 2026-09-23, tagged `v0.2.1`.** Not Responding
   and the CPU pin are fixed (bmLive stayed under 50ms throughout every measured run);
   peak WorkingSet dropped from ~1.4-1.7 GB to ~900 MB, short of the 400 MB Done-when
   target and left open in `STATUS.md`'s Known gaps for v0.3 or a follow-up patch.
5. v0.3 (2026-10-09, moved from 2026-12-12), next, one release, two sessions a week: price
   books, per-vendor cost, dev and business switch, client map (owner then client), active
   time, export and merge, Copilot VS Code (OTel file), macOS and Linux in browser mode.
   Slip order: platforms, then Copilot VS Code. Spec: `2026-09-23_v0.3_spec.md`. Session
   prompts with the checklist: `2026-09-23_v0.3_session_prompts.md`. Grilled 2026-09-23.
6. Decision 2026-12-19.

Full plan with hypotheses, risks and assumptions: `2026-09-22_burnmon_plan.md`.
Architecture (decided): `..\04_assets\2026-09-22_token_monitor_architecture.md`.
Now page and live features (decided): `..\04_assets\2026-09-22_burnmon_now_page_features.md`.
