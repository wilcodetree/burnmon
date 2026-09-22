// Package store is burnmon's local SQLite persistence: every parsed Event,
// one cursor per trail file (so a file is read from where it stopped, not
// from the start every time), and a small meta key/value table.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"burnmon/internal/schema"
)

type Store struct {
	db *sql.DB
}

// SchemaVersion is the current events/cursors schema. Open bumps a
// mismatched database back to it by dropping and recreating events and
// cursors (never meta): Step 1's events are cheaply re-derivable from the
// transcript files on disk, so a rebuild-from-scratch recovery is
// acceptable and simpler than a real migration.
const SchemaVersion = "1"

const schemaVersionKey = "schema_version"

const schemaDDL = `
CREATE TABLE IF NOT EXISTS events (
	vendor      TEXT NOT NULL,
	agent       TEXT NOT NULL,
	surface     TEXT NOT NULL,
	session_id  TEXT NOT NULL,
	request_id  TEXT NOT NULL,
	parent_id   TEXT NOT NULL DEFAULT '',
	at          TEXT NOT NULL,
	model       TEXT NOT NULL,
	project     TEXT NOT NULL DEFAULT '',
	title       TEXT NOT NULL DEFAULT '',
	input       INTEGER NOT NULL,
	cache_write INTEGER,
	cache_read  INTEGER,
	output      INTEGER NOT NULL,
	reasoning   INTEGER,
	vendor_cost REAL,
	window_used REAL,
	window_reset TEXT,
	tools       TEXT NOT NULL DEFAULT '{}',
	PRIMARY KEY (vendor, session_id, request_id)
);
CREATE INDEX IF NOT EXISTS idx_events_session ON events (vendor, session_id);
CREATE INDEX IF NOT EXISTS idx_events_at ON events (at);
CREATE TABLE IF NOT EXISTS cursors (
	path   TEXT PRIMARY KEY,
	offset INTEGER NOT NULL,
	mtime  TEXT NOT NULL,
	size   INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// Open creates path's parent folder if needed, opens (creating if absent)
// the SQLite file at path and applies the schema. Safe to call every run:
// every DDL statement is idempotent.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: create %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// One connection: modernc.org/sqlite serialises writes per connection
	// anyway, and burnmon's own callers (CLI: one-shot; app: one rebuild at
	// a time, guarded by app.building) never need concurrent writers.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaDDL); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate %s: %w", path, err)
	}
	s := &Store{db: db}
	if err := s.ensureSchemaVersion(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: schema version %s: %w", path, err)
	}
	return s, nil
}

// ensureSchemaVersion writes SchemaVersion into meta when absent (a fresh
// database), or, on a mismatch (an older code version's database), drops
// and recreates events and cursors so they get rebuilt from the transcript
// files on the next ingest, then records the current version.
func (s *Store) ensureSchemaVersion() error {
	v, ok, err := s.Meta(schemaVersionKey)
	if err != nil {
		return err
	}
	if ok && v == SchemaVersion {
		return nil
	}
	if ok && v != SchemaVersion {
		if _, err := s.db.Exec(`DROP TABLE IF EXISTS events`); err != nil {
			return fmt.Errorf("drop events: %w", err)
		}
		if _, err := s.db.Exec(`DROP TABLE IF EXISTS cursors`); err != nil {
			return fmt.Errorf("drop cursors: %w", err)
		}
		if _, err := s.db.Exec(schemaDDL); err != nil {
			return fmt.Errorf("recreate: %w", err)
		}
	}
	return s.SetMeta(schemaVersionKey, SchemaVersion)
}

func (s *Store) Close() error { return s.db.Close() }

// UpsertEvents inserts events, keeping, per (vendor, session_id, request_id),
// the one with the largest Output ("largest output wins": a streamed API
// call is written as several lines sharing a request id with only
// output_tokens growing, all within one session's ingest).
func (s *Store) UpsertEvents(events []schema.Event) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
INSERT INTO events (
	vendor, agent, surface, session_id, request_id, parent_id, at, model,
	project, title, input, cache_write, cache_read, output, reasoning,
	vendor_cost, window_used, window_reset, tools
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT (vendor, session_id, request_id) DO UPDATE SET
	agent = excluded.agent, surface = excluded.surface,
	session_id = excluded.session_id, parent_id = excluded.parent_id,
	at = excluded.at, model = excluded.model, project = excluded.project,
	title = excluded.title, input = excluded.input,
	cache_write = excluded.cache_write, cache_read = excluded.cache_read,
	output = excluded.output, reasoning = excluded.reasoning,
	vendor_cost = excluded.vendor_cost, window_used = excluded.window_used,
	window_reset = excluded.window_reset, tools = excluded.tools
WHERE excluded.output > events.output
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		toolsJSON, err := json.Marshal(e.Tools)
		if err != nil {
			return fmt.Errorf("store: marshal tools for %s/%s: %w", e.Vendor, e.RequestID, err)
		}
		var windowReset any
		if e.WindowReset != nil {
			windowReset = e.WindowReset.UTC().Format(time.RFC3339Nano)
		}
		_, err = stmt.Exec(
			e.Vendor, e.Agent, e.Surface, e.SessionID, e.RequestID, e.ParentID,
			e.At.UTC().Format(time.RFC3339Nano), e.Model, e.Project, e.Title,
			e.Input, nullInt(e.CacheWrite), nullInt(e.CacheRead), e.Output,
			nullInt(e.Reasoning), nullFloat(e.VendorCost), nullFloat(e.WindowUsed),
			windowReset, string(toolsJSON),
		)
		if err != nil {
			return fmt.Errorf("store: upsert %s/%s: %w", e.Vendor, e.RequestID, err)
		}
	}
	return tx.Commit()
}

func nullInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

const eventColumns = `vendor, agent, surface, session_id, request_id, parent_id, at, model,
	project, title, input, cache_write, cache_read, output, reasoning,
	vendor_cost, window_used, window_reset, tools`

// scanEvents reads every row of rows (already SELECTed with eventColumns'
// exact column list and order) into Events. Shared by AllEvents and
// EventsSince so the two queries' row-decoding logic (nullable columns,
// the tools JSON blob, RFC3339Nano timestamps) is written once.
func scanEvents(rows *sql.Rows) ([]schema.Event, error) {
	defer rows.Close()
	var out []schema.Event
	for rows.Next() {
		var e schema.Event
		var atStr string
		var cacheWrite, cacheRead, reasoning sql.NullInt64
		var vendorCost, windowUsed sql.NullFloat64
		var windowReset sql.NullString
		var toolsJSON string
		if err := rows.Scan(&e.Vendor, &e.Agent, &e.Surface, &e.SessionID, &e.RequestID,
			&e.ParentID, &atStr, &e.Model, &e.Project, &e.Title, &e.Input,
			&cacheWrite, &cacheRead, &e.Output, &reasoning, &vendorCost, &windowUsed,
			&windowReset, &toolsJSON); err != nil {
			return nil, err
		}
		var err error
		e.At, err = time.Parse(time.RFC3339Nano, atStr)
		if err != nil {
			return nil, fmt.Errorf("store: parse at %q: %w", atStr, err)
		}
		if cacheWrite.Valid {
			v := cacheWrite.Int64
			e.CacheWrite = &v
		}
		if cacheRead.Valid {
			v := cacheRead.Int64
			e.CacheRead = &v
		}
		if reasoning.Valid {
			v := reasoning.Int64
			e.Reasoning = &v
		}
		if vendorCost.Valid {
			v := vendorCost.Float64
			e.VendorCost = &v
		}
		if windowUsed.Valid {
			v := windowUsed.Float64
			e.WindowUsed = &v
		}
		if windowReset.Valid {
			t, err := time.Parse(time.RFC3339Nano, windowReset.String)
			if err == nil {
				e.WindowReset = &t
			}
		}
		if toolsJSON != "" && toolsJSON != "{}" {
			if err := json.Unmarshal([]byte(toolsJSON), &e.Tools); err != nil {
				return nil, fmt.Errorf("store: unmarshal tools: %w", err)
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AllEvents returns every stored event, oldest first by At. v0.1 scale
// (a developer's own machine, at most a few hundred thousand turns) makes
// loading the whole table fine for the CLI's one-shot report; the Now
// page's live poll uses EventsSince instead (see F1, SESSION_LOG.md), since
// polling AllEvents every 2 seconds re-groups the whole table in memory on
// every tick.
func (s *Store) AllEvents() ([]schema.Event, error) {
	rows, err := s.db.Query(`SELECT ` + eventColumns + ` FROM events ORDER BY at ASC`)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// EventsSince returns every stored event whose At is at or after from,
// oldest first, using the idx_events_at index. The Now page's live poll
// (bmLive) uses this with from = now - live.ChartWindow instead of
// AllEvents, so a 2-second poll only ever groups the chart window's worth
// of rows, not the whole history (F1: burnmon.exe was measured at 1,886 MB
// and thrashing before this fix).
func (s *Store) EventsSince(from time.Time) ([]schema.Event, error) {
	rows, err := s.db.Query(`SELECT `+eventColumns+` FROM events WHERE at >= ? ORDER BY at ASC`,
		from.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// SessionModelTotal is one (vendor, session_id, model) group's SQL-summed
// token counts, turn count and time span. A running session can switch
// model mid-session, so SessionTotals groups by model too rather than
// collapsing straight to one row per session; ApplySessionTotals (internal/live)
// sums cost per group (both cost formulas are linear in token counts, so
// this is exact, not an approximation) then adds the groups together.
type SessionModelTotal struct {
	Vendor     string
	SessionID  string
	Model      string
	Input      int64
	CacheWrite int64
	CacheRead  int64
	Output     int64
	Reasoning  int64
	Count      int64
	MinAt      time.Time
	MaxAt      time.Time
}

// SessionKey identifies one running session by (Vendor, SessionID): a bare
// session_id is not unique across vendors (two adapters can in principle
// mint the same id), so SessionTotals is looked up by the vendor-qualified
// pair rather than session_id alone.
type SessionKey struct {
	Vendor    string
	SessionID string
}

// SessionTotals returns one row per (vendor, session_id, model) group for
// every (vendor, session_id) pair in keys, SQL-summed across the whole
// events table (not windowed), so a session's lifetime Tokens/Cost/Start
// stay correct even though the Now page's live poll only re-reads the last
// ChartWindow of events for everything else (F1). The claude adapter's
// synthetic tool-only events (model="" and input=output=0) are excluded,
// matching internal/live's own isTurn filter. Returns nil, nil for an empty
// keys.
func (s *Store) SessionTotals(keys []SessionKey) ([]SessionModelTotal, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	clauses := make([]string, len(keys))
	args := make([]any, 0, len(keys)*2)
	for i, k := range keys {
		clauses[i] = "(vendor = ? AND session_id = ?)"
		args = append(args, k.Vendor, k.SessionID)
	}
	query := `
SELECT vendor, session_id, model,
	SUM(input) AS input, SUM(COALESCE(cache_write,0)) AS cache_write,
	SUM(COALESCE(cache_read,0)) AS cache_read, SUM(output) AS output,
	SUM(COALESCE(reasoning,0)) AS reasoning, COUNT(*) AS cnt,
	MIN(at) AS min_at, MAX(at) AS max_at
FROM events
WHERE (` + strings.Join(clauses, " OR ") + `)
	AND NOT (model = '' AND input = 0 AND output = 0)
GROUP BY vendor, session_id, model`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionModelTotal
	for rows.Next() {
		var t SessionModelTotal
		var minAtStr, maxAtStr string
		if err := rows.Scan(&t.Vendor, &t.SessionID, &t.Model, &t.Input, &t.CacheWrite,
			&t.CacheRead, &t.Output, &t.Reasoning, &t.Count, &minAtStr, &maxAtStr); err != nil {
			return nil, err
		}
		t.MinAt, err = time.Parse(time.RFC3339Nano, minAtStr)
		if err != nil {
			return nil, fmt.Errorf("store: parse min_at %q: %w", minAtStr, err)
		}
		t.MaxAt, err = time.Parse(time.RFC3339Nano, maxAtStr)
		if err != nil {
			return nil, fmt.Errorf("store: parse max_at %q: %w", maxAtStr, err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Cursor reports how far path has been read: ok is false when path has no
// cursor row yet (never seen before).
func (s *Store) Cursor(path string) (offset int64, mtime time.Time, size int64, ok bool, err error) {
	row := s.db.QueryRow(`SELECT offset, mtime, size FROM cursors WHERE path = ?`, path)
	var mtimeStr string
	err = row.Scan(&offset, &mtimeStr, &size)
	if err == sql.ErrNoRows {
		return 0, time.Time{}, 0, false, nil
	}
	if err != nil {
		return 0, time.Time{}, 0, false, err
	}
	mtime, err = time.Parse(time.RFC3339Nano, mtimeStr)
	if err != nil {
		return 0, time.Time{}, 0, false, err
	}
	return offset, mtime, size, true, nil
}

func (s *Store) SetCursor(path string, offset int64, mtime time.Time, size int64) error {
	_, err := s.db.Exec(`
INSERT INTO cursors (path, offset, mtime, size) VALUES (?, ?, ?, ?)
ON CONFLICT (path) DO UPDATE SET offset = excluded.offset, mtime = excluded.mtime, size = excluded.size`,
		path, offset, mtime.UTC().Format(time.RFC3339Nano), size)
	return err
}

// DeleteEventsForOtherPaths removes every stored event and cursor whose
// source transcript is not in keepPaths. burnmon reconstructs SessionID
// from a transcript's filename, but events carry no source-path column, so
// pruning by session id here would risk dropping a live session that
// merely shares an id pattern; instead the caller (dataset.ingest) is
// expected to pass every currently-resolved file path, and pruning targets
// cursors, whose path IS the source file, one-for-one with the events that
// file produced (same SessionID as its filename, per the claude adapter).
// A transcript that no longer exists (deleted, renamed, moved) therefore
// has its cursor removed here, and its events removed by SessionID in the
// same call, so it stops contributing to every future report.
func (s *Store) DeleteEventsForOtherPaths(keepPaths []string) error {
	rows, err := s.db.Query(`SELECT path FROM cursors`)
	if err != nil {
		return fmt.Errorf("store: list cursor paths: %w", err)
	}
	keep := make(map[string]bool, len(keepPaths))
	for _, p := range keepPaths {
		keep[p] = true
	}
	var stale []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return fmt.Errorf("store: scan cursor path: %w", err)
		}
		if !keep[p] {
			stale = append(stale, p)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: list cursor paths: %w", err)
	}
	rows.Close()
	if len(stale) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range stale {
		sessionID := sessionIDFromPath(p)
		if sessionID != "" {
			if _, err := tx.Exec(`DELETE FROM events WHERE session_id = ?`, sessionID); err != nil {
				return fmt.Errorf("store: delete events for %s: %w", p, err)
			}
		}
		if _, err := tx.Exec(`DELETE FROM cursors WHERE path = ?`, p); err != nil {
			return fmt.Errorf("store: delete cursor for %s: %w", p, err)
		}
	}
	return tx.Commit()
}

// sessionIDFromPath mirrors the claude adapter's own session id derivation
// (the transcript file's base name, extension stripped).
func sessionIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func (s *Store) Meta(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`
INSERT INTO meta (key, value) VALUES (?, ?)
ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// DefaultPath is %LOCALAPPDATA%\burnmon\burnmon.db on Windows, the same
// folder the parse cache and app log already use; ~/.config/burnmon/burnmon.db
// on macOS and Linux via os.UserConfigDir.
func DefaultPath() (string, error) {
	if runtime.GOOS == "windows" {
		if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
			return filepath.Join(lad, "burnmon", "burnmon.db"), nil
		}
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "burnmon", "burnmon.db"), nil
}
