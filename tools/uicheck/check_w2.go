package main

import (
	"fmt"
	"time"
)

// w2: the turn drawer must close via the X button, an overlay click, and
// Esc, opened both from a ticker line and (implicitly, same openTurnDrawer
// path) a chart bar.
//
// All three interactions are dispatched through the dev eval channel
// (el.click() / a real KeyboardEvent), not true hardware SendInput: this
// laptop runs an elevated terminal that Windows' UIPI (User Interface
// Privilege Isolation) will not let this non-elevated tool's window rank
// above in z-order, so a physical click/screen-capture at any fixed screen
// coordinate is not reliably burnmon's window (confirmed empirically:
// WindowFromPoint at burnmon's own supposed screen rect returned the
// terminal). el.click()/dispatchEvent still fire the exact same
// inline-onclick/addEventListener code paths a real click would (that is
// what W2's actual bug was: an inline onclick="..." attribute silently
// failing to resolve its function), so this remains a genuine functional
// check of the app, not a proxy/static one. Screenshots are still
// attempted for evidence but are best-effort: a failure is logged, not
// fatal, since it can be this same environment limitation rather than an
// app problem.
//
// Args: "before" only captures and reports; anything else (the default,
// "after") also asserts.
func init() {
	checks["w2"] = func(hwnd uintptr, args []string) error {
		phase := "after"
		if len(args) > 0 {
			phase = args[0]
		}

		bestEffortScreenshot := func(name string) {
			if _, err := screenshot(hwnd, name); err != nil {
				fmt.Printf("uicheck: screenshot %q failed (non-fatal, best effort only): %v\n", name, err)
			}
		}

		open := func(label string) error {
			if _, err := evalRaw(`document.querySelector('.ticker-line').click()`); err != nil {
				return fmt.Errorf("open (%s): %w", label, err)
			}
			ok, err := waitForOverlayOpen()
			if err != nil {
				return fmt.Errorf("open (%s): %w", label, err)
			}
			if !ok {
				return fmt.Errorf("open (%s): drawer overlay never opened", label)
			}
			bestEffortScreenshot("w2-" + phase + "-" + label + "-open")
			return nil
		}

		assertClosed := func(label string) error {
			time.Sleep(200 * time.Millisecond) // let the repaint settle before the best-effort screenshot
			bestEffortScreenshot("w2-" + phase + "-" + label + "-closed")
			hidden, err := overlayHidden()
			if err != nil {
				return fmt.Errorf("close (%s): %w", label, err)
			}
			fmt.Printf("uicheck: after %s, overlay hidden=%v\n", label, hidden)
			if phase != "before" && !hidden {
				return fmt.Errorf("close (%s): overlay still not hidden in the DOM", label)
			}
			return nil
		}

		// X button
		if err := open("x-button"); err != nil {
			return err
		}
		if _, err := evalRaw(`document.getElementById('turn_drawer_close').click()`); err != nil {
			return fmt.Errorf("click X: %w", err)
		}
		if err := assertClosed("x-button"); err != nil {
			return err
		}

		// Overlay click (event.target===this, i.e. the backdrop itself,
		// not the drawer card inside it)
		if err := open("overlay"); err != nil {
			return err
		}
		if _, err := evalRaw(`document.getElementById('turn_drawer_overlay').dispatchEvent(new MouseEvent('click', {bubbles: true}))`); err != nil {
			return fmt.Errorf("click overlay: %w", err)
		}
		if err := assertClosed("overlay"); err != nil {
			return err
		}

		// Esc
		if err := open("escape"); err != nil {
			return err
		}
		if _, err := evalRaw(`document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape'}))`); err != nil {
			return fmt.Errorf("press Escape: %w", err)
		}
		if err := assertClosed("escape"); err != nil {
			return err
		}

		return nil
	}
}

func overlayHidden() (bool, error) {
	return evalBool(`document.getElementById('turn_drawer_overlay').classList.contains('hidden')`)
}

func waitForOverlayOpen() (bool, error) {
	// openTurnDrawer removes 'hidden' synchronously (loading text shows
	// immediately, detail fills in async), so no polling/sleep is needed
	// beyond the click itself having landed.
	hidden, err := overlayHidden()
	if err != nil {
		return false, err
	}
	return !hidden, nil
}
