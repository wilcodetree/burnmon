//go:build !windows

package sysmon

// pdhState stub: PDH is Windows-only. Mirrors internal/collect/pressure_stub.go
// in perfadvisor, keeps the package portable for `go vet`/`go test` on a
// non-Windows dev machine.
type pdhState struct{}

type pressureTick struct {
	ok         bool
	cpuQueue   float64
	cpuPerfPct float64
	diskQueue  float64
	diskLatMs  float64
	hardFaults float64
	gpuPct     float64
}

func (s *pdhState) readTick() pressureTick { return pressureTick{} }
