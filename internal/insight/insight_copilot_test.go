package insight

import (
	"testing"
	"time"

	"burnmon/internal/schema"
)

// cev builds a Copilot in VS Code event: one OTel span per request, no cache
// fields, the way internal/adapter/copilotvsc stores them.
func cev(requestID, model string, at time.Time, fresh int64) schema.Event {
	return schema.Event{
		Vendor: "github", Agent: "copilot-vscode", Surface: "vscode",
		SessionID: "cp1", RequestID: requestID, Model: model,
		At: at, Input: fresh, Output: 50,
	}
}

func compactionsOnly(findings []Finding) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Kind == KindCompaction {
			out = append(out, f)
		}
	}
	return out
}

// TestAnalyze_Compaction_CopilotVSCodeHelperRequestsAreNotCompaction: a
// Copilot session interleaves the main conversation (a large, growing
// context) with small requests to a helper model (titles, summaries; about
// 250 to 3,800 tokens each, an independent prompt every time). The shape is
// session 4be14c72 from the real store, 2026-10-05. A helper request after a
// main request, and one helper request smaller than the previous helper
// request, are neither compactions: the main context only grew.
func TestAnalyze_Compaction_CopilotVSCodeHelperRequestsAreNotCompaction(t *testing.T) {
	base := time.Date(2026, 10, 5, 9, 15, 0, 0, time.UTC)
	const main, helper = "claude-sonnet-5.5", "gpt-4o-mini-2024-07-18"
	events := []schema.Event{
		cev("r1", helper, base, 1_677),
		cev("r2", main, base.Add(9*time.Second), 428_957),
		cev("r3", helper, base.Add(12*time.Second), 1_450), // helper after main: different model
		cev("r4", helper, base.Add(90*time.Second), 1_684),
		cev("r5", main, base.Add(100*time.Second), 465_676),
		cev("r6", helper, base.Add(105*time.Second), 1_351),
		cev("r7", helper, base.Add(10*time.Hour), 1_151),
		cev("r8", helper, base.Add(10*time.Hour+6*time.Second), 253), // helper after helper, 78% drop
		cev("r9", helper, base.Add(10*time.Hour+6*time.Second), 253),
	}
	if got := compactionsOnly(Analyze(events, testConfig())); len(got) != 0 {
		t.Fatalf("want no compaction findings in a Copilot session whose main context only grew, got %+v", got)
	}
}

// TestAnalyze_Compaction_CopilotVSCodeRealCompactionStillFires: the one real
// compaction in the store (session 20310c89, 2026-10-01): the main model's
// context went from 725,556 to 97,476 with helper requests in between. It
// fires once, on the main model's turn, measured against the main model's
// previous turn and not against the helper request before it.
func TestAnalyze_Compaction_CopilotVSCodeRealCompactionStillFires(t *testing.T) {
	base := time.Date(2026, 10, 1, 20, 11, 0, 0, time.UTC)
	const main, helper = "claude-sonnet-5.5", "gpt-4o-mini-2024-07-18"
	events := []schema.Event{
		cev("r1", main, base.Add(7*time.Second), 725_556),
		cev("r2", helper, base.Add(41*time.Second), 1_637),
		cev("r3", helper, base.Add(61*time.Second), 1_651),
		cev("r4", main, base.Add(62*time.Second), 97_476),
		cev("r5", helper, base.Add(66*time.Second), 2_159),
	}
	got := compactionsOnly(Analyze(events, testConfig()))
	if len(got) != 1 || got[0].Turn != 4 {
		t.Fatalf("want one compaction finding at turn 4, got %+v", got)
	}
	if got[0].Evidence["context_before"] != 725_556 || got[0].Evidence["context_after"] != 97_476 {
		t.Fatalf("want before 725556 and after 97476, got %+v", got[0].Evidence)
	}
}

// TestAnalyze_Compaction_OtherAgentsKeepTheAdjacentRule: the Copilot rule is
// scoped to Copilot in VS Code. Claude Code, Cowork and Codex keep comparing
// each turn with the one before it, across a model switch (opus then haiku in
// a Cowork session, 2026-10-05) and at any context size.
func TestAnalyze_Compaction_OtherAgentsKeepTheAdjacentRule(t *testing.T) {
	base := time.Date(2026, 10, 5, 13, 0, 0, 0, time.UTC)
	for _, agent := range []string{"claude-code", "cowork", "codex"} {
		t.Run(agent+" model switch", func(t *testing.T) {
			a := ev("s1", "r1", "claude-opus-5-5", base, 215_228, ptr(0), ptr(0), 500)
			b := ev("s1", "r2", "claude-haiku-4-5", base.Add(time.Minute), 58_651, ptr(0), ptr(0), 500)
			a.Agent, b.Agent = agent, agent
			got := compactionsOnly(Analyze([]schema.Event{a, b}, testConfig()))
			if len(got) != 1 || got[0].Turn != 2 {
				t.Fatalf("want one compaction finding at turn 2, got %+v", got)
			}
		})
		t.Run(agent+" small context", func(t *testing.T) {
			a := ev("s1", "r1", "claude-sonnet-5", base, 6_000, ptr(0), ptr(0), 500)
			b := ev("s1", "r2", "claude-sonnet-5", base.Add(time.Minute), 2_000, ptr(0), ptr(0), 500)
			a.Agent, b.Agent = agent, agent
			got := compactionsOnly(Analyze([]schema.Event{a, b}, testConfig()))
			if len(got) != 1 || got[0].Turn != 2 {
				t.Fatalf("want one compaction finding at turn 2, got %+v", got)
			}
		})
	}
}
