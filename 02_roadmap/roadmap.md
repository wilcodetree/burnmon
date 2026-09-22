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
   checklist: `2026-09-22_v0.2_session_prompts.md`. Grilled 2026-09-22.
4. v0.3 (2026-12-12): per-vendor cost and credits, dev and business switch (moved from v0.2,
   ships complete), full client map, active time, export and merge, macOS and Linux builds.
5. Decision 2026-12-19.

Full plan with hypotheses, risks and assumptions: `2026-09-22_burnmon_plan.md`.
Architecture (decided): `..\04_assets\2026-09-22_token_monitor_architecture.md`.
Now page and live features (decided): `..\04_assets\2026-09-22_burnmon_now_page_features.md`.
