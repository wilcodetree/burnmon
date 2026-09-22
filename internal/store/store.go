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
	"time"

	_ "modernc.org/sqlite"

	"burnmon/internal/schema"
)

type Store struct {
	db *sql.DB
}

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
	PRIMARY KEY (vendor, request_id)
);
CREATE INDEX IF NOT EXISTS idx_events_session ON events (vendor, session_id);
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
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// UpsertEvents inserts events, keeping, per (vendor, request_id), the one
// with the largest Output ("largest output wins": a streamed API call is
// written as several lines sharing a request id with only output_tokens
// growing).
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
ON CONFLICT (vendor, request_id) DO UPDATE SET
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

// AllEvents returns every stored event, oldest first by At. v0.1 scale
// (a developer's own machine, at most a few hundred thousand turns) makes
// loading the whole table fine; a windowed query can be added later if it
// ever needs to be.
func (s *Store) AllEvents() ([]schema.Event, error) {
	rows, err := s.db.Query(`
SELECT vendor, agent, surface, session_id, request_id, parent_id, at, model,
	project, title, input, cache_write, cache_read, output, reasoning,
	vendor_cost, window_used, window_reset, tools
FROM events ORDER BY at ASC`)
	if err != nil {
		return nil, err
	}
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
