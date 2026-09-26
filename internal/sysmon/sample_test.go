package sysmon

import "testing"

// TestSamplerTick is a light smoke test, not a behavioural spec: the real
// collectors (sample_windows.go) read live OS state, so this only checks
// that two ticks in a row don't error and don't report a negative rate,
// cross-platform (sample_stub.go on non-Windows).
func TestSamplerTick(t *testing.T) {
	s := NewSampler()
	first, err := s.Tick()
	if err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	if first.Ts.IsZero() {
		t.Error("first Tick did not set Ts")
	}

	second, err := s.Tick()
	if err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if second.DiskReadBps < 0 || second.DiskWriteBps < 0 || second.NetDownBps < 0 || second.NetUpBps < 0 {
		t.Errorf("second Tick reported a negative rate: %+v", second)
	}
}
