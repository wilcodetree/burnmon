package sysmon

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
	gpu_pct        REAL NOT NULL,
	cpu_queue      REAL NOT NULL DEFAULT -1,
	cpu_perf_pct   REAL NOT NULL DEFAULT -1,
	disk_queue_len REAL NOT NULL DEFAULT -1,
	disk_lat_ms    REAL NOT NULL DEFAULT -1,
	hard_faults    REAL NOT NULL DEFAULT -1
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

// pressureColumns is phase 3's additive migration for a sysmon_samples
// table created before the pressure signals existed (this workstream has
// no versioned migration table yet, unlike internal/store's schema_version:
// one small ALTER TABLE per new column, each ignored specifically for
// "duplicate column name", is enough for a store this size). A brand-new
// database never hits this path: the CREATE TABLE above already has every
// column.
var pressureColumns = []string{
	`ALTER TABLE sysmon_samples ADD COLUMN cpu_queue REAL NOT NULL DEFAULT -1`,
	`ALTER TABLE sysmon_samples ADD COLUMN cpu_perf_pct REAL NOT NULL DEFAULT -1`,
	`ALTER TABLE sysmon_samples ADD COLUMN disk_queue_len REAL NOT NULL DEFAULT -1`,
	`ALTER TABLE sysmon_samples ADD COLUMN disk_lat_ms REAL NOT NULL DEFAULT -1`,
	`ALTER TABLE sysmon_samples ADD COLUMN hard_faults REAL NOT NULL DEFAULT -1`,
}

func addPressureColumns(db *sql.DB) error {
	for _, stmt := range pressureColumns {
		if _, err := db.Exec(stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("sysmon: %s: %w", stmt, err)
		}
	}
	return nil
}

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
	if err := addPressureColumns(db); err != nil {
		db.Close()
		return nil, err
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
			(ts, cpu_pct, cores, mem_used_mb, mem_total_mb, disk_read_bps, disk_write_bps, net_down_bps, net_up_bps, gpu_pct,
			 cpu_queue, cpu_perf_pct, disk_queue_len, disk_lat_ms, hard_faults)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ts) DO UPDATE SET
			cpu_pct=excluded.cpu_pct, cores=excluded.cores, mem_used_mb=excluded.mem_used_mb,
			mem_total_mb=excluded.mem_total_mb, disk_read_bps=excluded.disk_read_bps,
			disk_write_bps=excluded.disk_write_bps, net_down_bps=excluded.net_down_bps,
			net_up_bps=excluded.net_up_bps, gpu_pct=excluded.gpu_pct,
			cpu_queue=excluded.cpu_queue, cpu_perf_pct=excluded.cpu_perf_pct,
			disk_queue_len=excluded.disk_queue_len, disk_lat_ms=excluded.disk_lat_ms,
			hard_faults=excluded.hard_faults`,
		sm.Ts.UnixMilli(), sm.CPUPct, string(cores), sm.MemUsedMB, sm.MemTotalMB,
		sm.DiskReadBps, sm.DiskWriteBps, sm.NetDownBps, sm.NetUpBps, sm.GPUPct,
		sm.CPUQueue, sm.CPUPerfPct, sm.DiskQueueLen, sm.DiskLatMs, sm.HardFaults)
	if err != nil {
		return fmt.Errorf("sysmon: insert sample: %w", err)
	}
	return nil
}

// RecentSamples returns every sample at or after since, oldest first.
func (s *Store) RecentSamples(since time.Time) ([]Sample, error) {
	rows, err := s.db.Query(`
		SELECT ts, cpu_pct, cores, mem_used_mb, mem_total_mb, disk_read_bps, disk_write_bps, net_down_bps, net_up_bps, gpu_pct,
			cpu_queue, cpu_perf_pct, disk_queue_len, disk_lat_ms, hard_faults
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
			&sm.DiskReadBps, &sm.DiskWriteBps, &sm.NetDownBps, &sm.NetUpBps, &sm.GPUPct,
			&sm.CPUQueue, &sm.CPUPerfPct, &sm.DiskQueueLen, &sm.DiskLatMs, &sm.HardFaults); err != nil {
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
