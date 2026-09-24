// Package hermes reads Hermes's own SQLite state database (WAL mode) and
// turns its sessions into schema.Events.
//
// Confirmed live on Wilco's laptop (2026-09-22, SESSION_LOG.md, v0.2 41B):
// %LOCALAPPDATA%\Hermes\state.db, table `sessions` carries one row per
// session with running-total columns (input_tokens, output_tokens,
// cache_read_tokens, cache_write_tokens, reasoning_tokens, message_count)
// that grow in place as the session continues; table `messages` exists but
// its own `token_count` column is null on every row observed (24/24, every
// role), so no per-message token breakdown is available at all, contrary to
// the v0.2 spec's "per-message token count" assumption. There is no
// context-window column anywhere in the schema.
//
// Design (decided in the 41B session, in place of "one Event per message"):
// one Event per session, re-emitted on every poll with the session's current
// running totals, RequestID fixed to the session id so the store's own
// "largest output wins" upsert (UpsertEvents, ON CONFLICT ... WHERE
// excluded.output > events.output) naturally keeps only the latest state and
// skips a no-op write when a session has not grown since the last poll. This
// means PollOnce needs no cursor of its own: it is safe to call it with the
// same dbPath every 5 seconds and hand every row it returns straight to
// UpsertEvents.
//
// Known trade-off, not solved here: schema.Event's Input/CacheWrite/
// CacheRead/Output are meant to be one turn's consumption; for Hermes they
// are instead the session's lifetime running totals (the only numbers this
// schema exposes). internal/live's Session.Context (last turn's context
// fill, used for the Now page's gauge) sums exactly those fields, so a long
// Hermes session's gauge reads as ever-growing lifetime usage, not current
// context fill, and can appear to exceed its model's context window even
// when the live conversation is far smaller. Accepted for v0.2: Hermes
// writes no tool_calls and carries no context-window column either, so the
// gauge already falls back to "context window unknown" for it in practice
// (see contextWindowKnown below); this only matters if a future price-book
// entry happens to key on the same model string Hermes reports.
package hermes

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"

	"burnmon/internal/schema"
)

// DefaultDBPath is $HERMES_HOME/state.db when HERMES_HOME is set (Hermes's
// own override), else the platform default: %LOCALAPPDATA%\Hermes\state.db
// on Windows (confirmed live on Wilco's laptop, 2026-09-22, SESSION_LOG.md
// v0.2 41B), ~/.hermes/state.db on macOS and Linux. The macOS/Linux default
// was V3-5's VERIFY (guessed as ~/Library/Application Support/Hermes and
// $XDG_DATA_HOME/Hermes, following the per-OS app-data convention
// store.DefaultPath uses); live-checked 2026-09-24 against Hermes's own
// docs (github.com/NousResearch/hermes-agent, session-storage.md: "the
// HERMES_HOME environment variable, and finally the platform default
// (~/.hermes on macOS and Linux; %LOCALAPPDATA%/hermes on Windows)") and
// that guess was wrong: Hermes does not follow either platform's app-data
// convention, it always uses one dot-folder in $HOME regardless of OS.
// Empty (not an error) when the path cannot be resolved or the file does
// not exist, so a caller can treat "no Hermes installed" the same as "no
// roots" elsewhere in burnmon.
func DefaultDBPath() string {
	var dir string
	switch {
	case os.Getenv("HERMES_HOME") != "":
		dir = os.Getenv("HERMES_HOME")
	case runtime.GOOS == "windows":
		if la := os.Getenv("LOCALAPPDATA"); la != "" {
			dir = filepath.Join(la, "Hermes")
		}
	default: // darwin, linux, and anything else pure-Go builds target
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = filepath.Join(home, ".hermes")
		}
	}
	if dir == "" {
		return ""
	}
	p := filepath.Join(dir, "state.db")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// surfaceOf maps a Hermes sessions.source value to burnmon's surface
// vocabulary. Only "tui" has been observed live; anything else is
// "unknown" rather than guessed.
func surfaceOf(source string) string {
	if source == "tui" {
		return "cli"
	}
	return "unknown"
}

// PollOnce opens dbPath read-only (WAL mode: a concurrent Hermes write never
// blocks or corrupts this read) and returns one Event per session that has
// exchanged at least one message, current as of this call. No offset or
// cursor: call this every 5 seconds (A1's poll interval; Hermes gets no
// fsnotify watch, since a SQLite WAL file's own writes do not fit the
// existing watch.Watcher's file-offset, .jsonl-only design) and pass every
// row straight to store.UpsertEvents.
func PollOnce(dbPath string) ([]schema.Event, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=busy_timeout(2000)")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
SELECT id, source, model, parent_session_id, started_at, ended_at, cwd,
	input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, reasoning_tokens
FROM sessions
WHERE message_count > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	var events []schema.Event
	for rows.Next() {
		var id, source, model, cwd string
		var parentID sql.NullString
		var startedAt float64
		var endedAt sql.NullFloat64
		var input, output, cacheRead, cacheWrite, reasoning int64
		if err := rows.Scan(&id, &source, &model, &parentID, &startedAt, &endedAt, &cwd,
			&input, &output, &cacheRead, &cacheWrite, &reasoning); err != nil {
			return nil, err
		}
		_ = startedAt // kept for a future "session start" need; not used today

		// At tracks "current state as of this poll": now while the session
		// is still open (ended_at unset), so a session mid-conversation
		// keeps landing inside internal/live's running window as it grows;
		// its own ended_at once Hermes has recorded an end. Neither is a
		// real per-turn timestamp (see the package doc's trade-off note).
		at := now
		if endedAt.Valid {
			at = unixSeconds(endedAt.Float64)
		}

		events = append(events, schema.Event{
			Vendor:     "nous",
			Agent:      "hermes",
			Surface:    surfaceOf(source),
			SessionID:  id,
			RequestID:  id, // one row per session; unique within (vendor, session_id) trivially
			ParentID:   parentID.String,
			At:         at,
			Model:      model,
			Project:    cwd,
			Input:      input,
			CacheWrite: &cacheWrite,
			CacheRead:  &cacheRead,
			Output:     output,
			Reasoning:  &reasoning,
		})
	}
	return events, rows.Err()
}

func unixSeconds(f float64) time.Time {
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC()
}
