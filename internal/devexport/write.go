package devexport

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteResult is Write's report: the folder it created and each file's
// path and size in bytes, so a caller can print or bind back the real
// sizes (phase 4's own Done-when: "report the real sizes of all three
// files for a 24-hour export").
type WriteResult struct {
	Dir           string
	SummaryPath   string
	DataJSONPath  string
	DailyCSVPath  string
	SummaryBytes  int
	DataJSONBytes int
	DailyCSVBytes int
}

// Write creates outDir\burnmon-dev_<yyyy-mm-dd_hhmmss>\ (the timestamp
// taken from b.GeneratedAt, in the local zone, so a caller that fixed
// GeneratedAt for a test gets a deterministic folder name) and writes
// summary.md, data.json and daily.csv into it. The only function in this
// package that touches disk; every Build* function above stays a pure
// string/bytes builder. Local time down to the second, not GeneratedAt's
// own UTC (every other timestamp in the bundle is UTC, but a folder name
// is what a person reads at a glance): minute-only UTC collided when two
// exports landed in the same UTC minute, silently overwriting the first
// (found by review, 2026-09-24).
func Write(outDir string, b Bundle) (WriteResult, error) {
	folder := "burnmon-dev_" + b.GeneratedAt.Local().Format("2006-01-02_150405")
	dir := filepath.Join(outDir, folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return WriteResult{}, fmt.Errorf("devexport: create %s: %w", dir, err)
	}

	summary := BuildSummaryMD(b)
	dataJSON, err := BuildDataJSON(b)
	if err != nil {
		return WriteResult{}, fmt.Errorf("devexport: build data.json: %w", err)
	}
	daily := BuildDailyCSV(b)

	res := WriteResult{Dir: dir}

	res.SummaryPath = filepath.Join(dir, "summary.md")
	if err := os.WriteFile(res.SummaryPath, []byte(summary), 0o644); err != nil {
		return WriteResult{}, fmt.Errorf("devexport: write summary.md: %w", err)
	}
	res.SummaryBytes = len(summary)

	res.DataJSONPath = filepath.Join(dir, "data.json")
	if err := os.WriteFile(res.DataJSONPath, dataJSON, 0o644); err != nil {
		return WriteResult{}, fmt.Errorf("devexport: write data.json: %w", err)
	}
	res.DataJSONBytes = len(dataJSON)

	res.DailyCSVPath = filepath.Join(dir, "daily.csv")
	if err := os.WriteFile(res.DailyCSVPath, []byte(daily), 0o644); err != nil {
		return WriteResult{}, fmt.Errorf("devexport: write daily.csv: %w", err)
	}
	res.DailyCSVBytes = len(daily)

	return res, nil
}
