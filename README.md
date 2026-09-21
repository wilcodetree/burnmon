# claudecost

Your own Claude usage and cost dashboard, as a portable Windows app or a
single-file CLI. It reads the session transcripts Cowork and Claude Code
already write on your machine, prices them, and shows a self-contained HTML
dashboard: current and previous month, by month, week, day and session, in
euros, plus a Tools tab showing which connectors, plugins and skills your
sessions actually call.

This is the standalone sibling of the "Claude Cost (User)" Cowork dashboard
plugin: same data, same dedup rules, same dashboard page, but no Cowork
session, no mirror folder, no scheduled tasks. Everything reads the
transcript folders directly on this device.

Everything stays on this device. No API key, no admin rights, no network
calls, no install.

## The app (recommended)

Double-click `claudecost.exe`. One window opens, no console, no tray
icon, no browser tab. It collects your usage on startup, again every 30
minutes while the window stays open, and whenever you press "Refresh now" in
the bottom-right corner. Close the window and the app is gone; nothing keeps
running in the background.

- No open ports, no admin rights (`asInvoker`).
- Needs the WebView2 runtime, which ships with Edge on Windows 10/11. If it
  is somehow missing, the app falls back to writing the report once and
  opening it in your default browser instead.
- Checks for a `claudecost.json` next to the exe first (so the exe and the
  config can be dropped together into one portable folder), then falls back
  to `%LOCALAPPDATA%\claudecost\claudecost.json` for a fixed install that
  must run from a read-only share. Cache and log always live under
  `%LOCALAPPDATA%\claudecost\`, regardless of which config was used.
- Launching it a second time brings the existing window to the front instead
  of opening a second copy.
- Flags: `-interval` (default 15m, floor 5m), `-wsl-interval` (how often to
  re-read WSL transcripts specifically, default 4h, floor 15m; see "WSL"
  below), `-months` (default 2), `-seat`, `-config`, `-source` (repeatable).
  Same meaning as the CLI flags below.
- Parsed transcripts are cached in `%LOCALAPPDATA%\claudecost\parsecache.gob`,
  so only the first start (and the first after an update or a config change)
  reads every transcript; later starts show the previous dashboard right
  away and quietly refresh in the background. Delete that file to force a
  full re-read.
- Click the gear icon (top right, next to Refresh now) to edit your
  subscription numbers, seat counts and seat prices from inside the window,
  no more hand-editing `claudecost.json`. Saving re-reads everything, since
  cached sessions carry costs computed with the old numbers. Settings saves
  land back in whichever `claudecost.json` was actually loaded, exe-adjacent
  or `%LOCALAPPDATA%`, not always the latter.
- Set `"your_seat"` in `claudecost.json` (e.g. `"Premium"`) to default the
  "Your seat" dropdown for everyone who runs this config, useful when a
  whole team shares one seat tier. An explicit `-seat` flag still wins.

Windows will likely show a blue "Windows protected your PC" screen the first
time, since this is an unsigned binary. That is expected: click **More info**,
then **Run anyway**, or build it yourself from source (see Build below).

## The CLI

For power users and scripting. Run it, and it writes a timestamped HTML
report and opens it once, then exits.

    claudecost                  build the dashboard and open it in the browser
    claudecost -months 3        widen the window to three months
    claudecost -seat Premium    price your seat tier correctly (default Standard,
                                 or your_seat from claudecost.json if set)
    claudecost -no-open         write the report without opening it
    claudecost -json data.json  also write the raw dataset
    claudecost -source DIR      scan DIR instead of the auto-detected folders
    claudecost -no-cache        skip the parse cache, always re-parse everything

Reports land in a `reports` folder next to the exe if writable, otherwise in
`%LOCALAPPDATA%\claudecost\reports`. Override with `-out DIR`.

## What it does

- Finds transcripts in the known locations (Claude Code `~\.claude\projects`,
  Cowork under `%APPDATA%` or the MSIX `%LOCALAPPDATA%\Packages` path, plus
  the macOS and Linux equivalents, plus WSL, see "WSL" below).
- Deduplicates streamed API calls (one entry per requestId, largest
  output_tokens), drops synthetic turns, and prices every call two ways:
  subscription share (what a flat monthly plan actually costs, allocated by
  consumption) and API list price (the comparison).
- Renders the dashboard with Chart.js vendored inline: no network access.
- The CLI also prints a terminal summary with per-month totals.

Coverage is Cowork and Claude Code only. claude.ai in the browser keeps no
local transcript, so it cannot appear here; check claude.ai/settings/usage
for those limit bars instead.

## WSL

Claude Code running inside a WSL distribution (Ubuntu under WSL, most
commonly) is a Linux process with a Linux `$HOME`, so its transcripts live
inside the distro, not under any Windows folder. As of v0.7.0 claudecost
finds them automatically: it reads which WSL distributions are installed
from the registry, then looks for each one's `.claude/projects` folder,
including one moved via a `CLAUDE_CONFIG_DIR` set in a shell startup file.

A few things are worth knowing:

- Reading Linux files from Windows goes over WSL's own file-sharing layer
  and is slower than reading a native NTFS folder, so a scan that includes
  WSL can take longer than one that does not. To keep this from slowing down
  the app's regular refresh, WSL transcripts are re-read on their own,
  slower clock (`-wsl-interval`, default 4 hours) while native Windows
  transcripts keep the normal 15-minute one. Refresh now and saving Settings
  always re-read everything, WSL included. When WSL data is present, the
  header notice at the top of the dashboard states both cadences, and a
  small "WSL data as of HH:MM" stamp next to the main snapshot time shows
  how current the WSL half actually is.
- Accessing a stopped WSL 2 distribution's files can start it in the
  background, which costs a few seconds and some memory. This only happens
  on the slower WSL cadence above, not on every 15-minute refresh.
- To turn WSL scanning off entirely, set `"wsl_scan": "off"` in
  `claudecost.json`. This is also the first thing to try if a scan seems
  slow and you want to rule WSL out.
- If detection misses a distro, or a config directory set somewhere other
  than a shell startup file (a systemd unit, a wrapper script, an IDE launch
  config), add the folder directly with `"extra_sources": ["..."]` in
  `claudecost.json`, or with a repeatable `-source` flag. Unlike `-source`
  on its own, `extra_sources` adds to auto-detection rather than replacing
  it.
- A native Windows install with a moved `CLAUDE_CONFIG_DIR` is also picked
  up now, independent of WSL.

See `claudecost.example.json` for the exact config keys.

## Build

Requires Go (winget install GoLang.Go). Then, in PowerShell, from this
folder:

    .\build.ps1

First run needs internet access: it fetches `go-winres` for the icons and
`go mod tidy` resolves the WebView2 binding. Produces `claudecost-cli.exe`
(the CLI) and `claudecost.exe` (the app), both portable single files. Copy
them anywhere; no install. If a `build.local.ps1` exists next to `build.ps1`
(gitignored, machine-local), it runs afterward; use it for any private
post-build step such as copying the built app into another folder.

## Configuration

Prices and the subscription calibration are compiled in as illustrative
defaults (see `internal\pricing\pricing.go`), not a real invoice. Plug in
your own numbers one of two ways.

In the app, click the gear icon and fill in the form: your subscription
total, seat counts, seat prices, usage credits, and (behind an Advanced
section, since it changes far less often) the output cost factor and
calibration date. Saving writes `claudecost.json` for you and re-reads
everything.

Or edit the file directly, useful for the CLI, for scripting, or for
anything outside the subscription block, such as an unusual per-model price
override or the WSL settings described above: drop a `claudecost.json` next
to the exe, CLI or app alike, or pass `-config` explicitly, or (app only)
place it in `%LOCALAPPDATA%\claudecost\` for a fixed, read-only-share
install. Overrides any subset of the defaults. See `claudecost.example.json`.
The recommended calibration source is your real invoice total divided by
consumption, not seats times list price, since a flat subscription rarely
maps cleanly to per-seat usage.

## Status

Personal project, actively used daily. The dashboard is deliberately
personal: it reads only your own transcript folders and never aggregates or
compares across people. Costs shown are allocated shares of a flat monthly
invoice, not money owed by anyone.
