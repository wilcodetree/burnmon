package main

import (
	"fmt"
	"time"
)

// d0: phase 4's advisor panel ("TODAY'S READ") is present in the DOM and
// bdevAdvisorNow actually populates it (or, honestly, its own empty state)
// rather than staying stuck on nothing. Primary viewport.
func init() {
	checks["d0"] = func(hwnd uintptr, args []string) error {
		if err := ensureWindowSizeWH(hwnd, 1152, 2048); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)

		html, err := evalString(`document.getElementById('advisorFindings').outerHTML`)
		if err != nil {
			return fmt.Errorf("read advisor panel: %w", err)
		}
		if html == "" {
			return fmt.Errorf("advisor panel element not found")
		}

		// Give bdevAdvisorNow's async resolve a moment to land, then require
		// the panel to hold either real finding rows or its own documented
		// empty state, never a blank/never-rendered container.
		var inner string
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			inner, err = evalString(`document.getElementById('advisorFindings').innerHTML`)
			if err != nil {
				return fmt.Errorf("read advisor panel contents: %w", err)
			}
			if inner != "" {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		if inner == "" {
			return fmt.Errorf("advisor panel never rendered anything (neither findings nor the empty state)")
		}
		fmt.Println("uicheck: advisor panel rendered", len(inner), "bytes of content")

		if _, err := screenshot(hwnd, "d0-advisor-panel"); err != nil {
			return err
		}
		return nil
	}
}
