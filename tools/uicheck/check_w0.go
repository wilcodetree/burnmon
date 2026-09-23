package main

import (
	"fmt"
	"strings"
)

// w0: prove the harness itself works against the real window: screenshot it
// (pure Win32, no cooperation from burnmon.exe needed) and read a piece of
// its DOM back through the dev eval channel (the Now tab's own button,
// which template.html marks "on" when Now is the active tab, the default).
func init() {
	checks["w0"] = func(hwnd uintptr, args []string) error {
		if _, err := screenshot(hwnd, "w0-now"); err != nil {
			return err
		}
		html, err := evalString(`document.querySelector('button[data-t="now"]').outerHTML`)
		if err != nil {
			return err
		}
		if !strings.Contains(html, "Now") {
			return fmt.Errorf("Now tab button DOM did not contain \"Now\": %s", html)
		}
		fmt.Println("uicheck: read Now tab button DOM:", html)
		return nil
	}
}
