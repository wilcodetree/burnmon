// internal\schema\event_test.go
package schema

import "testing"

func TestEventZeroValueHasNilOptionalFields(t *testing.T) {
	var e Event
	if e.CacheWrite != nil || e.CacheRead != nil || e.Reasoning != nil ||
		e.VendorCost != nil || e.WindowUsed != nil || e.WindowReset != nil {
		t.Fatal("Event's optional pointer fields must be nil on the zero value")
	}
	if e.Tools != nil {
		t.Fatal("Event.Tools must be nil on the zero value, not an empty map")
	}
}
