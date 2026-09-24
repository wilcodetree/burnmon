//go:build !windows

package sysmon

import "time"

// Sampler stub: BurnMon Dev is Windows-only (WebView2, screen 2/screen 1
// only exist on Wilco's own laptop), this keeps the package portable for
// `go vet`/`go test` on a non-Windows dev machine.
type Sampler struct{}

func NewSampler() *Sampler { return &Sampler{} }

func (s *Sampler) Tick() (Sample, error) {
	return Sample{Ts: time.Now()}, nil
}
