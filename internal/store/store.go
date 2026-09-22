// Package store is burnmon's local SQLite persistence: every parsed Event,
// one cursor per trail file (so a file is read from where it stopped, not
// from the start every time), and a small meta key/value table.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"burnmon/internal/schema"
	"burnmon/internal/store/migrations"
)

type Store struct {
	db *sql.DB
}

const schemaVersionDDL = `CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);`

// Open creates path's parent folder if needed, opens (creating if absent)
// the SQLite file at path, and runs every pending migration (migrations.All)
// inside one transaction, logging the from/to version when it moved. Safe to
// call every run: every migration is additive and idempotent against a
// database already at or past its version.
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
	from, to, err := runMigrations(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate %s: %w", path, err)
	}
	if from != to {
		log.Printf("store: migrated %s from version %d to %d", path, from, to)
	}
	return &Store{db: db}, nil
}

// runMigrations ensures the schema_version table exists, reads the store's
// current version (0 for a database that predates this table, including
// every real v0.1 store on disk), and runs every migration whose Version is
// greater than that, in order, inside one transaction, recording the head
// version on success.
func runMigrations(db *sql.DB) (from, to int, err error) {
	if _, err = db.Exec(schemaVersionDDL); err != nil {
		return 0, 0, fmt.Errorf("create schema_version: %w", err)
	}
	var current int
	err = db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&current)
	if err == sql.ErrNoRows {
		current, err = 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("read schema_version: %w", err)
	}
	from = current

	var pending []migrations.Migration
	for _, m := range migrations.All {
		if m.Version > current {
			pending = append(pending, m)
		}
	}
	if len(pending) == 0 {
		return from, from, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return from, from, err
	}
	defer tx.Rollback()
	for _, m := range pending {
		if err = m.Up(tx); err != nil {
			return from, from, fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}
	}
	to = pending[len(pending)-1].Version
	if _, err = tx.Exec(`DELETE FROM schema_version`); err != nil {
		return from, from, err
	}
	if _, err = tx.Exec(`INSERT INTO schema_version (version) VALUES (?)`, to); err != nil {
		return from, from, err
	}
	if err = tx.Commit(); err != nil {
		return from, from, err
	}
	return from, to, nil
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
	vendor_cost, window_used, window_reset, tools, owner
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT (vendor, session_id, request_id) DO UPDATE SET
	agent = excluded.agent, surface = excluded.surface,
	session_id = excluded.session_id, parent_id = excluded.parent_id,
	at = excluded.at, model = excluded.model, project = excluded.project,
	title = excluded.title, input = excluded.input,
	cache_write = excluded.cache_write, cache_read = excluded.cache_read,
	output = excluded.output, reasoning = excluded.reasoning,
	vendor_cost = excluded.vendor_cost, window_used = excluded.window_used,
	window_reset = excluded.window_reset, tools = excluded.tools,
	owner = excluded.owner
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
			windowReset, string(toolsJSON), e.Owner,
		)
		if err != nil {
			return fmt.Errorf("store: upsert %s/%s: %w", e.Vendor, e.RequestID, err)
		}
	}
	return tx.Commit()
}

// UpsertToolCalls inserts tool calls, keyed by (vendor, session_id, call_id).
// A repeat call for the same call_id (a later incremental read of the same
// file re-encountering the row, or the call and its result arriving in
// separate Parse calls) updates every column but only overwrites
// result_bytes when the new row actually carries one, so a result already
// recorded is never wiped back to unknown by a later row that has none.
func (s *Store) UpsertToolCalls(calls []schema.ToolCall) error {
	if len(calls) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
INSERT INTO tool_calls (vendor, agent, session_id, call_id, turn, tool, at, input_bytes, result_bytes)
VALUES (?,?,?,?,?,?,?,?,?)
ON CONFLICT (vendor, session_id, call_id) DO UPDATE SET
	agent = excluded.agent, turn = excluded.turn, tool = excluded.tool, at = excluded.at,
	input_bytes = excluded.input_bytes,
	result_bytes = COALESCE(excluded.result_bytes, tool_calls.result_bytes)
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range calls {
		_, err = stmt.Exec(
			c.Vendor, c.Agent, c.SessionID, c.CallID, c.Turn, c.Tool,
			c.At.UTC().Format(time.RFC3339Nano), c.InputBytes, nullInt(c.ResultBytes),
		)
		if err != nil {
			return fmt.Errorf("store: upsert tool call %s/%s: %w", c.Vendor, c.CallID, err)
		}
	}
	return tx.Commit()
}

// ToolCallTotal is one tool name's aggregated activity since a cutoff, as
// `burnmon-cli tools` and (later) the Tools tab both need it.
type ToolCallTotal struct {
	Tool        string `json:"tool"`
	Calls       int64  `json:"calls"`
	Sessions    int64  `json:"sessions"`
	InputBytes  int64  `json:"input_bytes"`
	ResultBytes int64  `json:"result_bytes"`
}

// ToolCallTotals returns one row per tool name for every call at or after
// since, ordered by call count descending. result_bytes sums only the calls
// that have one recorded; a tool whose calls never carried a result (an
// orphaned call_id, see schema.ToolCall) still gets a row, with
// result_bytes 0.
func (s *Store) ToolCallTotals(since time.Time) ([]ToolCallTotal, error) {
	rows, err := s.db.Query(`
SELECT tool, COUNT(*) AS calls, COUNT(DISTINCT session_id) AS sessions,
	SUM(input_bytes) AS input_bytes, SUM(COALESCE(result_bytes, 0)) AS result_bytes
FROM tool_calls
WHERE at >= ?
GROUP BY tool
ORDER BY calls DESC`, since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ToolCallTotal
	for rows.Next() {
		var t ToolCallTotal
		if err := rows.Scan(&t.Tool, &t.Calls, &t.Sessions, &t.InputBytes, &t.ResultBytes); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// VendorStripTotal is one agent's token totals for the Now page's vendor
// strip (v0.2 P3): today, this week and this month, all as of the caller's
// day/week/month boundaries.
type VendorStripTotal struct {
	Agent string
	Today int64
	Week  int64
	Month int64
}

// VendorStripTotals runs one SQL query, grouped by agent, summing each
// event's token count (input + cache_write + cache_read + output) into
// today/week/month buckets via CASE, rather than loading events into Go and
// summing there (P3: refreshed once a minute, from its own timer, not
// bmLive's 2-second poll). from bounds the scan to the earliest of the three
// boundaries the caller passes (the current ISO week can start before the
// current calendar month, so it is not always monthStart).
func (s *Store) VendorStripTotals(day, week, month time.Time) ([]VendorStripTotal, error) {
	from := day
	if week.Before(from) {
		from = week
	}
	if month.Before(from) {
		from = month
	}
	rows, err := s.db.Query(`
SELECT agent,
	SUM(CASE WHEN at >= ? THEN input + COALESCE(cache_write,0) + COALESCE(cache_read,0) + output ELSE 0 END) AS today,
	SUM(CASE WHEN at >= ? THEN input + COALESCE(cache_write,0) + COALESCE(cache_read,0) + output ELSE 0 END) AS week,
	SUM(CASE WHEN at >= ? THEN input + COALESCE(cache_write,0) + COALESCE(cache_read,0) + output ELSE 0 END) AS month
FROM events
WHERE at >= ?
GROUP BY agent`,
		day.UTC().Format(time.RFC3339Nano), week.UTC().Format(time.RFC3339Nano),
		month.UTC().Format(time.RFC3339Nano), from.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []VendorStripTotal
	for rows.Next() {
		var t VendorStripTotal
		if err := rows.Scan(&t.Agent, &t.Today, &t.Week, &t.Month); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
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
	vendor_cost, window_used, window_reset, tools, owner`

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
			&windowReset, &toolsJSON, &e.Owner); err != nil {
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

// EventsForSession returns every stored event for sessionID, any vendor,
// oldest first: `burnmon-cli insight <session-id>` (v0.2 I1) reads one
// session's whole history this way rather than through AllEvents, the same
// "windowed, not whole-table" instinct EventsSince already follows for the
// Now page. A bare session_id is not guaranteed unique across vendors
// (SessionKey's own doc comment), so a session id shared by two vendors
// returns both; insight.Analyze still groups by turn order within the
// result, which read as multiplexed turns of two sessions is a display bug,
// not a correctness one, no vendor has ever repeated another's id in
// practice.
func (s *Store) EventsForSession(sessionID string) ([]schema.Event, error) {
	rows, err := s.db.Query(`SELECT `+eventColumns+` FROM events WHERE session_id = ? ORDER BY at ASC`,
		sessionID)
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

// ReownEvents recomputes every event's owner via ownerFor(project) (P6's
// light client map, re-run after an owner rule change) and updates every
// row whose owner actually changes, in one transaction. Returns the number
// of rows updated. Used by `burnmon-cli reown`.
func (s *Store) ReownEvents(ownerFor func(project string) string) (int, error) {
	rows, err := s.db.Query(`SELECT vendor, session_id, request_id, project, owner FROM events`)
	if err != nil {
		return 0, fmt.Errorf("store: list events for reown: %w", err)
	}
	type update struct {
		vendor, sessionID, requestID, owner string
	}
	var updates []update
	for rows.Next() {
		var vendor, sessionID, requestID, project, owner string
		if err := rows.Scan(&vendor, &sessionID, &requestID, &project, &owner); err != nil {
			rows.Close()
			return 0, fmt.Errorf("store: scan event for reown: %w", err)
		}
		if want := ownerFor(project); want != owner {
			updates = append(updates, update{vendor, sessionID, requestID, want})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("store: list events for reown: %w", err)
	}
	rows.Close()
	if len(updates) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`UPDATE events SET owner = ? WHERE vendor = ? AND session_id = ? AND request_id = ?`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, u := range updates {
		if _, err := stmt.Exec(u.owner, u.vendor, u.sessionID, u.requestID); err != nil {
			return 0, fmt.Errorf("store: reown %s/%s: %w", u.vendor, u.requestID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(updates), nil
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
