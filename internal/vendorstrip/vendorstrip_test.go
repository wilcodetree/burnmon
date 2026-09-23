package vendorstrip

import (
	"path/filepath"
	"testing"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/schema"
	"burnmon/internal/store"
)

// TestBuildTwoVendors is P3's spec'd test: a store fixture with two vendors
// (claude-code, codex) with events today, earlier this week, earlier this
// month and before the month started, checked against hand-computed
// today/week/month totals; the earlier-than-month event must never appear.
func TestBuildTwoVendors(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// A Wednesday, comfortably inside its ISO week and its calendar month,
	// with room before it for a "week but not today" and a "month but not
	// week" event, and room before the month started for an excluded one.
	now := time.Date(2026, 9, 23, 15, 0, 0, 0, time.UTC) // Wednesday
	today := time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC)
	earlierThisWeek := time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)  // Monday
	earlierThisMonth := time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)  // before the week started
	beforeThisMonth := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)  // last month, excluded

	mk := func(vendor, agent, id string, at time.Time, input, output int64) schema.Event {
		return schema.Event{
			Vendor: vendor, Agent: agent, SessionID: id, RequestID: id + "-" + at.Format("20060102150405"),
			At: at, Model: "m", Input: input, Output: output,
		}
	}

	events := []schema.Event{
		mk("anthropic", "claude-code", "c1", today, 100, 50),
		mk("anthropic", "claude-code", "c2", earlierThisWeek, 200, 20),
		mk("anthropic", "claude-code", "c3", earlierThisMonth, 300, 30),
		mk("anthropic", "claude-code", "c4", beforeThisMonth, 9000, 9000),
		mk("openai", "codex", "x1", today, 10, 5),
		mk("openai", "codex", "x2", earlierThisMonth, 40, 4),
	}
	if err := st.UpsertEvents(events); err != nil {
		t.Fatal(err)
	}

	cfg := pricing.Defaults()
	payload, err := Build(st, &cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	if payload.CopilotCreditsLeft != nil {
		t.Fatalf("CopilotCreditsLeft = %+v, want nil with no CopilotPlan configured", payload.CopilotCreditsLeft)
	}
	if len(payload.Rows) != 2 {
		t.Fatalf("rows = %d, want 2, got %+v", len(payload.Rows), payload.Rows)
	}

	byAgent := map[string]Row{}
	for _, r := range payload.Rows {
		byAgent[r.Agent] = r
	}

	cc := byAgent["claude-code"]
	if want := int64(150); cc.Today != want {
		t.Errorf("claude-code today = %d, want %d", cc.Today, want)
	}
	if want := int64(150 + 220); cc.Week != want {
		t.Errorf("claude-code week = %d, want %d", cc.Week, want)
	}
	if want := int64(150 + 220 + 330); cc.Month != want {
		t.Errorf("claude-code month = %d, want %d", cc.Month, want)
	}
	if cc.AgentLabel != "Claude Code" {
		t.Errorf("claude-code label = %q, want %q", cc.AgentLabel, "Claude Code")
	}

	cx := byAgent["codex"]
	if want := int64(15); cx.Today != want {
		t.Errorf("codex today = %d, want %d", cx.Today, want)
	}
	if want := int64(15); cx.Week != want {
		t.Errorf("codex week = %d, want %d", cx.Week, want)
	}
	if want := int64(15 + 44); cx.Month != want {
		t.Errorf("codex month = %d, want %d", cx.Month, want)
	}

	wantTotalToday := cc.Today + cx.Today
	wantTotalWeek := cc.Week + cx.Week
	wantTotalMonth := cc.Month + cx.Month
	if payload.Total.Today != wantTotalToday || payload.Total.Week != wantTotalWeek || payload.Total.Month != wantTotalMonth {
		t.Errorf("total = %+v, want today=%d week=%d month=%d", payload.Total, wantTotalToday, wantTotalWeek, wantTotalMonth)
	}
	if payload.Total.AgentLabel != "Total" {
		t.Errorf("total label = %q, want %q", payload.Total.AgentLabel, "Total")
	}
}

// TestBuildCopilotCreditsLeft guards C3/K3's account-wide balance: present
// only once Config.CopilotPlan names a known plan, computed from this
// calendar month's github-vendor events.
func TestBuildCopilotCreditsLeft(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "burnmon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	now := time.Date(2026, 9, 23, 15, 0, 0, 0, time.UTC)
	cw, cr := int64(0), int64(0)
	events := []schema.Event{{
		Vendor: "github", Agent: "copilot-cli", SessionID: "g1", RequestID: "g1-1",
		At: now, Model: "claude-sonnet-5", Input: 1_000_000, Output: 1_000_000,
		CacheWrite: &cw, CacheRead: &cr,
	}}
	if err := st.UpsertEvents(events); err != nil {
		t.Fatal(err)
	}

	cfg := pricing.Defaults()
	var planName string
	var monthly float64
	for name, p := range cfg.CopilotCredits.Plans {
		planName, monthly = name, p.MonthlyCredits
		break
	}
	if planName == "" {
		t.Skip("no Copilot plan in the compiled-in book to test against")
	}
	cfg.CopilotPlan = planName

	payload, err := Build(st, &cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	if payload.CopilotCreditsLeft == nil {
		t.Fatal("CopilotCreditsLeft = nil, want a value with CopilotPlan configured")
	}
	if payload.CopilotCreditsLeft.Plan != planName {
		t.Errorf("Plan = %q, want %q", payload.CopilotCreditsLeft.Plan, planName)
	}
	if payload.CopilotCreditsLeft.MonthlyCredits != monthly {
		t.Errorf("MonthlyCredits = %v, want %v", payload.CopilotCreditsLeft.MonthlyCredits, monthly)
	}
}
