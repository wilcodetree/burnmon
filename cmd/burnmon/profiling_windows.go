//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessHandleCount = kernel32.NewProc("GetProcessHandleCount")
)

// processHandleCount returns the current process's open handle count
// (GetProcessHandleCount), or -1 if the call fails.
func processHandleCount() int {
	var count uint32
	r, _, _ := procGetProcessHandleCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&count)))
	if r == 0 {
		return -1
	}
	return int(count)
}
