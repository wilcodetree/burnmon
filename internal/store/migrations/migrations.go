// Package migrations lists burnmon's store schema migrations in order.
// Every migration is additive only: add a table, add a column, or backfill
// data; none may drop or rewrite the events table. internal/store.Open runs
// every migration whose Version is greater than the store's recorded
// version, all inside one transaction, then records the new version.
package migrations

import "database/sql"

// Migration is one numbered, additive schema step.
type Migration struct {
	Version int
	Name    string
	Up      func(tx *sql.Tx) error
}

// All is every migration, in order. Version numbers start at 1 and are
// contiguous.
var All = []Migration{
	{1, "baseline v0.1 schema (events, cursors, meta)", migration1},
	{2, "owner column on events (P6 owner split)", migration2},
	{3, "tool_calls table (S2)", migration3},
	{4, "tool_calls.path column (I3)", migration4},
	{5, "forecast_scores table (F1)", migration5},
}

// migration1 records the v0.1 schema as version 1 without changing it: the
// same CREATE TABLE IF NOT EXISTS statements store.go always ran on every
// startup, so a real v0.1 database on disk (already at this shape) is left
// untouched and a brand-new database gets the same tables.
func migration1(tx *sql.Tx) error {
	_, err := tx.Exec(`
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
`)
	return err
}

// migration2 adds P6's owner column: the light client map is applied at
// ingest and stored per event, empty by default, so a store with no owner
// rules configured carries owner = '' on every row.
func migration2(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE events ADD COLUMN owner TEXT NOT NULL DEFAULT ''`)
	return err
}

// migration3 adds S2's tool_calls table: one row per tool invocation, keyed
// by the vendor's own call id so a call and its later-arriving result can be
// upserted onto the same row. result_bytes is nullable: a call whose result
// was never seen in the same read (see schema.ToolCall) keeps it nil rather
// than 0, so "no result yet" stays distinguishable from "an empty result".
func migration3(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS tool_calls (
	vendor       TEXT NOT NULL,
	agent        TEXT NOT NULL,
	session_id   TEXT NOT NULL,
	call_id      TEXT NOT NULL,
	turn         TEXT NOT NULL,
	tool         TEXT NOT NULL,
	at           TEXT NOT NULL,
	input_bytes  INTEGER NOT NULL,
	result_bytes INTEGER,
	PRIMARY KEY (vendor, session_id, call_id)
);
CREATE INDEX IF NOT EXISTS idx_tool_calls_at ON tool_calls (at);
CREATE INDEX IF NOT EXISTS idx_tool_calls_tool ON tool_calls (tool);
`)
	return err
}

// migration4 adds I3's tool_calls.path column: the file path a Read, Edit,
// Write or NotebookEdit call touched, when the adapter could extract one
// from the tool's own input, "" otherwise (a shell/exec call, a search, or a
// trail whose input shape was not recognised).
func migration4(tx *sql.Tx) error {
	_, err := tx.Exec(`ALTER TABLE tool_calls ADD COLUMN path TEXT NOT NULL DEFAULT ''`)
	return err
}

// migration5 adds F1's forecast_scores table: one row per ISO week, the
// plan_tokens recorded the first time that week is seen (meant to be
// Monday, but a store that starts scoring mid-week records it the first
// time it runs instead of waiting for next Monday), actual_tokens and
// scored_at filled in once, after that week has fully elapsed. A week
// counts as scored (the forecast gate's condition) once both columns are
// non-null.
func migration5(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS forecast_scores (
	iso_year      INTEGER NOT NULL,
	iso_week      INTEGER NOT NULL,
	week_start    TEXT NOT NULL,
	plan_tokens   INTEGER NOT NULL,
	actual_tokens INTEGER,
	scored_at     TEXT,
	PRIMARY KEY (iso_year, iso_week)
);
`)
	return err
}
