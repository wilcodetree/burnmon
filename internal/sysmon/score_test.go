package sysmon

import "testing"

func TestPressureScore(t *testing.T) {
	tests := []struct {
		name string
		s    Sample
		want int
	}{
		{
			name: "idle machine scores near zero",
			s: Sample{
				CPUPct: 0, Cores: []float64{0, 0, 0, 0},
				MemUsedMB: 0, MemTotalMB: 16000,
				DiskLatMs: 0, CPUQueue: 0, GPUPct: 0,
			},
			want: 0,
		},
		{
			name: "everything maxed scores 100",
			s: Sample{
				CPUPct: 100, Cores: []float64{100, 100, 100, 100},
				MemUsedMB: 16000, MemTotalMB: 16000,
				DiskLatMs: 40, CPUQueue: 16, GPUPct: 100,
			},
			want: 100,
		},
		{
			name: "every component at its own halfway point scores 50",
			s: Sample{
				CPUPct: 50, Cores: []float64{50, 50, 50, 50}, // 4 cores
				MemUsedMB: 8000, MemTotalMB: 16000, // 50% used
				DiskLatMs: 10, // half of the 20ms reference
				CPUQueue:  4,  // half of 2*cores=8
				GPUPct:    50,
			},
			want: 50,
		},
		{
			name: "unavailable counters (-1) don't poison the average",
			s: Sample{
				CPUPct: 100, Cores: []float64{100},
				MemUsedMB: 1000, MemTotalMB: 1000, // 100%
				DiskLatMs: -1, CPUQueue: -1, GPUPct: -1,
			},
			// only cpu (0.30) and mem (0.20) contribute, both maxed: 50/100
			// of the total weight is present, all of it at 100 -> still 100
			// once renormalized against the weight actually present, not
			// silently averaged against phantom zeros for the missing ones.
			want: 100,
		},
		{
			name: "zero cores excludes the cpu-queue component instead of dividing by zero",
			s:    Sample{CPUPct: 20, Cores: nil, MemUsedMB: 100, MemTotalMB: 1000, CPUQueue: 5},
			// cpu-queue's weight (0.20) is excluded entirely (no core count to
			// normalize against), so the remaining weight (cpu 0.30, disk 0.20
			// at its zero-value default, mem 0.20, gpu 0.10 at its zero-value
			// default) is renormalized: (0.30*20 + 0.20*0 + 0.20*10 + 0.10*0) / 0.80 = 10
			want: 10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PressureScore(tc.s)
			if got != tc.want {
				t.Errorf("PressureScore(%+v) = %d, want %d", tc.s, got, tc.want)
			}
			if got < 0 || got > 100 {
				t.Errorf("PressureScore(%+v) = %d, out of [0,100]", tc.s, got)
			}
		})
	}
}
