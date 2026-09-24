package sysmon

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

// Store is burnmon-dev.db: system samples only, its own file and its own
// schema, so it never touches burnmon.db's own schema or migrations (design
// doc section 6). One writer connection, same WAL/busy-timeout recipe as
// internal/store, though contention here is moot: nothing else ever writes
// to this file.
type Store struct {
	db *sql.DB
}

// DefaultPath mirrors internal/store.DefaultPath's own per-OS split, one
// file name over: "<appDataDir>\burnmon-dev.db" next to burnmon.db.
func DefaultPath() (string, error) {
	if runtime.GOOS == "windows" {
		if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
			return filepath.Join(lad, "burnmon", "burnmon-dev.db"), nil
		}
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "burnmon", "burnmon-dev.db"), nil
}

const schema = `
CREATE TABLE IF NOT EXISTS sysmon_samples (
	ts             INTEGER PRIMARY KEY,
	cpu_pct        REAL NOT NULL,
	cores          TEXT NOT NULL,
	mem_used_mb    REAL NOT NULL,
	mem_total_mb   REAL NOT NULL,
	disk_read_bps  REAL NOT NULL,
	disk_write_bps REAL NOT NULL,
	net_down_bps   REAL NOT NULL,
	net_up_bps     REAL NOT NULL,
	gpu_pct        REAL NOT NULL
);
CREATE TABLE IF NOT EXISTS process_group_samples (
	ts       INTEGER NOT NULL,
	harness  TEXT NOT NULL,
	cpu_pct  REAL NOT NULL,
	mem_mb   REAL NOT NULL,
	io_bps   REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_process_group_samples_ts ON process_group_samples(ts);
`

// Open creates path's parent folder if needed and opens (creating if
// absent) the SQLite file at path, then ensures both tables exist. Safe to
// call every run: the DDL above is all IF NOT EXISTS, there are no
// versioned migrations yet (see the design doc: no schema change ships in
// this workstream beyond the initial tables).
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("sysmon: create %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sysmon: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("sysmon: journal_mode=WAL: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("sysmon: busy_timeout: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sysmon: create schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// InsertSample stores one system-wide tick. A tick at a ts already stored
// (the 10s persistence cadence landing on the same second twice) replaces
// it rather than erroring, since ts is the primary key.
func (s *Store) InsertSample(sm Sample) error {
	cores, err := json.Marshal(sm.Cores)
	if err != nil {
		return fmt.Errorf("sysmon: marshal cores: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO sysmon_samples
			(ts, cpu_pct, cores, mem_used_mb, mem_total_mb, disk_read_bps, disk_write_bps, net_down_bps, net_up_bps, gpu_pct)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ts) DO UPDATE SET
			cpu_pct=excluded.cpu_pct, cores=excluded.cores, mem_used_mb=excluded.mem_used_mb,
			mem_total_mb=excluded.mem_total_mb, disk_read_bps=excluded.disk_read_bps,
			disk_write_bps=excluded.disk_write_bps, net_down_bps=excluded.net_down_bps,
			net_up_bps=excluded.net_up_bps, gpu_pct=excluded.gpu_pct`,
		sm.Ts.UnixMilli(), sm.CPUPct, string(cores), sm.MemUsedMB, sm.MemTotalMB,
		sm.DiskReadBps, sm.DiskWriteBps, sm.NetDownBps, sm.NetUpBps, sm.GPUPct)
	if err != nil {
		return fmt.Errorf("sysmon: insert sample: %w", err)
	}
	return nil
}

// RecentSamples returns every sample at or after since, oldest first.
func (s *Store) RecentSamples(since time.Time) ([]Sample, error) {
	rows, err := s.db.Query(`
		SELECT ts, cpu_pct, cores, mem_used_mb, mem_total_mb, disk_read_bps, disk_write_bps, net_down_bps, net_up_bps, gpu_pct
		FROM sysmon_samples WHERE ts >= ? ORDER BY ts ASC`, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("sysmon: query samples: %w", err)
	}
	defer rows.Close()

	var out []Sample
	for rows.Next() {
		var sm Sample
		var ts int64
		var cores string
		if err := rows.Scan(&ts, &sm.CPUPct, &cores, &sm.MemUsedMB, &sm.MemTotalMB,
			&sm.DiskReadBps, &sm.DiskWriteBps, &sm.NetDownBps, &sm.NetUpBps, &sm.GPUPct); err != nil {
			return nil, fmt.Errorf("sysmon: scan sample: %w", err)
		}
		sm.Ts = time.UnixMilli(ts).UTC()
		if err := json.Unmarshal([]byte(cores), &sm.Cores); err != nil {
			return nil, fmt.Errorf("sysmon: unmarshal cores: %w", err)
		}
		out = append(out, sm)
	}
	return out, rows.Err()
}

// InsertProcessGroupSamples stores one row per harness for one tick.
func (s *Store) InsertProcessGroupSamples(rows []ProcessGroupSample) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("sysmon: begin: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO process_group_samples (ts, harness, cpu_pct, mem_mb, io_bps) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("sysmon: prepare: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.Ts.UnixMilli(), string(r.Harness), r.CPUPct, r.MemMB, r.IOBps); err != nil {
			return fmt.Errorf("sysmon: insert process group sample: %w", err)
		}
	}
	return tx.Commit()
}

// RecentProcessGroups returns every process-group row at or after since,
// oldest first.
func (s *Store) RecentProcessGroups(since time.Time) ([]ProcessGroupSample, error) {
	rows, err := s.db.Query(`
		SELECT ts, harness, cpu_pct, mem_mb, io_bps FROM process_group_samples
		WHERE ts >= ? ORDER BY ts ASC`, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("sysmon: query process groups: %w", err)
	}
	defer rows.Close()

	var out []ProcessGroupSample
	for rows.Next() {
		var r ProcessGroupSample
		var ts int64
		var harness string
		if err := rows.Scan(&ts, &harness, &r.CPUPct, &r.MemMB, &r.IOBps); err != nil {
			return nil, fmt.Errorf("sysmon: scan process group: %w", err)
		}
		r.Ts = time.UnixMilli(ts).UTC()
		r.Harness = Harness(harness)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Prune deletes every row older than cutoff from both tables. Called once a
// day by the app (retention_days in burnmon-dev.json, default 7, per the
// design doc).
func (s *Store) Prune(cutoff time.Time) error {
	if _, err := s.db.Exec(`DELETE FROM sysmon_samples WHERE ts < ?`, cutoff.UnixMilli()); err != nil {
		return fmt.Errorf("sysmon: prune samples: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM process_group_samples WHERE ts < ?`, cutoff.UnixMilli()); err != nil {
		return fmt.Errorf("sysmon: prune process groups: %w", err)
	}
	return nil
}
