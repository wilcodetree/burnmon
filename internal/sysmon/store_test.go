package sysmon

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "burnmon-dev.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestOpenCreatesSchema(t *testing.T) {
	st := openTestStore(t)
	if st == nil {
		t.Fatal("Open returned a nil store")
	}
}

func TestInsertAndRecentSamples(t *testing.T) {
	st := openTestStore(t)
	base := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		s := Sample{
			Ts:           base.Add(time.Duration(i) * 10 * time.Second),
			CPUPct:       float64(10 + i),
			Cores:        []float64{float64(i), float64(i + 1)},
			MemUsedMB:    1000,
			MemTotalMB:   16000,
			DiskReadBps:  1,
			DiskWriteBps: 2,
			NetDownBps:   3,
			NetUpBps:     4,
			GPUPct:       5,
		}
		if err := st.InsertSample(s); err != nil {
			t.Fatalf("InsertSample %d: %v", i, err)
		}
	}

	got, err := st.RecentSamples(base.Add(-time.Minute))
	if err != nil {
		t.Fatalf("RecentSamples: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("RecentSamples returned %d rows, want 3", len(got))
	}
	if got[0].CPUPct != 10 || got[2].CPUPct != 12 {
		t.Errorf("CPUPct order/values wrong: %+v", got)
	}
	if len(got[1].Cores) != 2 || got[1].Cores[0] != 1 || got[1].Cores[1] != 2 {
		t.Errorf("Cores round-trip wrong: %+v", got[1].Cores)
	}
	if !got[0].Ts.Equal(base) {
		t.Errorf("Ts round-trip wrong: got %v want %v", got[0].Ts, base)
	}

	// since in the future returns nothing.
	future, err := st.RecentSamples(base.Add(time.Hour))
	if err != nil {
		t.Fatalf("RecentSamples future: %v", err)
	}
	if len(future) != 0 {
		t.Errorf("RecentSamples with a future 'since' returned %d rows, want 0", len(future))
	}
}

func TestInsertAndRecentProcessGroups(t *testing.T) {
	st := openTestStore(t)
	base := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

	rows := []ProcessGroupSample{
		{Ts: base, Harness: HarnessClaude, CPUPct: 14, MemMB: 210, IOBps: 400000},
		{Ts: base, Harness: HarnessCodex, CPUPct: 38, MemMB: 612, IOBps: 6100000},
		{Ts: base.Add(10 * time.Second), Harness: HarnessClaude, CPUPct: 16, MemMB: 220, IOBps: 410000},
	}
	if err := st.InsertProcessGroupSamples(rows); err != nil {
		t.Fatalf("InsertProcessGroupSamples: %v", err)
	}

	got, err := st.RecentProcessGroups(base.Add(-time.Minute))
	if err != nil {
		t.Fatalf("RecentProcessGroups: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("RecentProcessGroups returned %d rows, want 3", len(got))
	}

	var codexRow *ProcessGroupSample
	for i := range got {
		if got[i].Harness == HarnessCodex {
			codexRow = &got[i]
		}
	}
	if codexRow == nil || codexRow.CPUPct != 38 {
		t.Errorf("codex row missing or wrong: %+v", got)
	}
}

func TestPruneRemovesOldRows(t *testing.T) {
	st := openTestStore(t)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

	old := Sample{Ts: now.Add(-10 * 24 * time.Hour), CPUPct: 1}
	recent := Sample{Ts: now.Add(-1 * time.Hour), CPUPct: 2}
	if err := st.InsertSample(old); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertSample(recent); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertProcessGroupSamples([]ProcessGroupSample{
		{Ts: old.Ts, Harness: HarnessClaude, CPUPct: 1},
		{Ts: recent.Ts, Harness: HarnessClaude, CPUPct: 2},
	}); err != nil {
		t.Fatal(err)
	}

	cutoff := now.Add(-7 * 24 * time.Hour)
	if err := st.Prune(cutoff); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	samples, err := st.RecentSamples(now.Add(-30 * 24 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0].CPUPct != 2 {
		t.Errorf("Prune left wrong sysmon_samples rows: %+v", samples)
	}

	groups, err := st.RecentProcessGroups(now.Add(-30 * 24 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].CPUPct != 2 {
		t.Errorf("Prune left wrong process_group_samples rows: %+v", groups)
	}
}
