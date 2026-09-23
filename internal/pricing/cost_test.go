package pricing

import (
	"testing"

	"burnmon/internal/schema"
)

func int64p(v int64) *int64 { return &v }

// anthropicEvent, openAIEvent and copilotEvent build one representative
// event per vendor, sized so the expected cost is easy to hand-check.
func anthropicEvent(model string) schema.Event {
	return schema.Event{
		Vendor: "anthropic", Model: model,
		Input: 1_000_000, CacheWrite: int64p(1_000_000), CacheRead: int64p(1_000_000), Output: 1_000_000,
	}
}

func openAIEvent(model string) schema.Event {
	return schema.Event{
		Vendor: "openai", Model: model,
		Input: 1_000_000, CacheRead: int64p(1_000_000), Output: 1_000_000,
	}
}

func copilotEvent(model string) schema.Event {
	return schema.Event{
		Vendor: "github", Model: model,
		Input: 1_000_000, CacheWrite: int64p(1_000_000), CacheRead: int64p(1_000_000), Output: 1_000_000,
	}
}

func findVendor(t *testing.T, vcs []VendorCost, vendor string) VendorCost {
	t.Helper()
	for _, vc := range vcs {
		if vc.Vendor == vendor {
			return vc
		}
	}
	t.Fatalf("no VendorCost for vendor %q in %+v", vendor, vcs)
	return VendorCost{}
}

func findBasis(t *testing.T, vc VendorCost, basis Basis) BasisCost {
	t.Helper()
	for _, b := range vc.Bases {
		if b.Basis == basis {
			return b
		}
	}
	t.Fatalf("vendor %q has no %q basis in %+v", vc.Vendor, basis, vc.Bases)
	return BasisCost{}
}

// TestCostForEvents_AnthropicAPIListBasis guards C2's Anthropic API-list
// fixture: 1M input, 1M 5-minute cache write, 1M cache read, 1M output on
// Claude Sonnet 5 (books/anthropic_api.json: in 2, cache_write_5m 2.50,
// cache_read 0.20, out 10, all USD/MTok) totals 2 + 2.50 + 0.20 + 10 = 14.70.
func TestCostForEvents_AnthropicAPIListBasis(t *testing.T) {
	cfg := Defaults()
	got := cfg.CostForEvents([]schema.Event{anthropicEvent("claude-sonnet-5")})
	vc := findVendor(t, got, "anthropic")
	b := findBasis(t, vc, BasisAPIList)
	want := 2.0 + 2.50 + 0.20 + 10.0
	if diff := b.USD - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("anthropic API list USD = %v, want %v", b.USD, want)
	}
}

// TestCostForEvents_AnthropicSubscriptionBasis guards the subscription-share
// basis: only present once Subscription is configured (not Defaults'
// illustrative example), output-driven plus fresh input, times
// OutputCostFactor.
func TestCostForEvents_AnthropicSubscriptionBasis(t *testing.T) {
	cfg := Defaults()
	if _, ok := hasBasis(cfg.CostForEvents([]schema.Event{anthropicEvent("claude-sonnet-5")}), "anthropic", BasisSubscription); ok {
		t.Fatal("subscription basis must not appear before Subscription is configured")
	}

	cfg.Subscription.CalibratedOn = "2026-09-01"
	cfg.Subscription.OutputCostFactor = 2.0
	got := cfg.CostForEvents([]schema.Event{anthropicEvent("claude-sonnet-5")})
	vc := findVendor(t, got, "anthropic")
	b := findBasis(t, vc, BasisSubscription)
	// Claude Sonnet 5: in 2, out 10, USD/MTok. (2*1 + 10*1) * 2.0 = 24.
	want := 24.0
	if diff := b.USD - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("anthropic subscription USD = %v, want %v", b.USD, want)
	}
}

// TestCostForEvents_OpenAIAPIListBasis guards the Codex/OpenAI basis:
// books/openai_api.json's gpt-6-astra (in 10, cached_in 1, out 50, USD/MTok)
// over 1M input, 1M cache-read, 1M output: 10 + 1 + 50 = 61.
func TestCostForEvents_OpenAIAPIListBasis(t *testing.T) {
	cfg := Defaults()
	got := cfg.CostForEvents([]schema.Event{openAIEvent("gpt-6-astra")})
	vc := findVendor(t, got, "openai")
	b := findBasis(t, vc, BasisAPIList)
	want := 61.0
	if diff := b.USD - want; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("openai API list USD = %v, want %v", b.USD, want)
	}
}

// TestCostForEvents_CopilotCreditsBasis guards the GitHub Copilot basis:
// books/copilot_credits.json's claude-sonnet-5 (in 2, cached_in 0.20,
// cache_write 2.50, out 10, USD-equivalent/MTok) over 1M input, 1M cache
// write, 1M cache read, 1M output: 2 + 2.50 + 0.20 + 10 = 14.70 USD, i.e.
// 1470 credits at $0.01 each.
func TestCostForEvents_CopilotCreditsBasis(t *testing.T) {
	cfg := Defaults()
	got := cfg.CostForEvents([]schema.Event{copilotEvent("claude-sonnet-5")})
	vc := findVendor(t, got, "github")
	b := findBasis(t, vc, BasisCredits)
	wantUSD := 14.70
	if diff := b.USD - wantUSD; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("copilot credits USD = %v, want %v", b.USD, wantUSD)
	}
	wantCredits := 1470.0
	if diff := b.Credits - wantCredits; diff > 1e-6 || diff < -1e-6 {
		t.Fatalf("copilot credits = %v, want %v", b.Credits, wantCredits)
	}
}

// TestCostForEvents_HeadlineCopilotIsCredits guards the headline rule's
// Copilot-specific branch: credits, even though no subscription applies to
// GitHub Copilot.
func TestCostForEvents_HeadlineCopilotIsCredits(t *testing.T) {
	cfg := Defaults()
	got := cfg.CostForEvents([]schema.Event{copilotEvent("claude-sonnet-5")})
	vc := findVendor(t, got, "github")
	if vc.Headline == nil || vc.Headline.Basis != BasisCredits {
		t.Fatalf("github headline = %+v, want credits", vc.Headline)
	}
}

// TestCostForEvents_HeadlineSubscriptionWhenConfigured guards the second
// headline rule: subscription share wins over API list once configured.
func TestCostForEvents_HeadlineSubscriptionWhenConfigured(t *testing.T) {
	cfg := Defaults()
	cfg.Subscription.CalibratedOn = "2026-09-01"
	got := cfg.CostForEvents([]schema.Event{anthropicEvent("claude-sonnet-5")})
	vc := findVendor(t, got, "anthropic")
	if vc.Headline == nil || vc.Headline.Basis != BasisSubscription {
		t.Fatalf("anthropic headline (configured) = %+v, want subscription", vc.Headline)
	}
}

// TestCostForEvents_HeadlineAPIListUpperBoundByDefault guards the fallback
// rule: with no subscription configured, the headline is API list, labelled
// "upper bound" per spec 4.3.
func TestCostForEvents_HeadlineAPIListUpperBoundByDefault(t *testing.T) {
	cfg := Defaults()
	got := cfg.CostForEvents([]schema.Event{anthropicEvent("claude-sonnet-5")})
	vc := findVendor(t, got, "anthropic")
	if vc.Headline == nil || vc.Headline.Basis != BasisAPIList {
		t.Fatalf("anthropic headline (unconfigured) = %+v, want api_list", vc.Headline)
	}
	if vc.Headline.Label != "API list (upper bound)" {
		t.Fatalf("anthropic headline label = %q, want %q", vc.Headline.Label, "API list (upper bound)")
	}
}

// TestCostForEvents_VendorWithNoBookIsTokensOnly guards Hermes ("nous") and
// any other vendor with no book at all: no bases, nil headline, "tokens
// only".
func TestCostForEvents_VendorWithNoBookIsTokensOnly(t *testing.T) {
	cfg := Defaults()
	got := cfg.CostForEvents([]schema.Event{{Vendor: "nous", Model: "hermes-1", Input: 1000, Output: 1000}})
	vc := findVendor(t, got, "nous")
	if len(vc.Bases) != 0 {
		t.Fatalf("nous (Hermes) Bases = %+v, want none (tokens only)", vc.Bases)
	}
	if vc.Headline != nil {
		t.Fatalf("nous (Hermes) Headline = %+v, want nil (tokens only)", vc.Headline)
	}
}

// TestCostForEvents_UnknownModelPricesZero guards a model id the live check
// never confirmed (no book entry): it prices at 0 rather than guessing, and
// does not panic or drop the vendor's other bases.
func TestCostForEvents_UnknownModelPricesZero(t *testing.T) {
	cfg := Defaults()
	got := cfg.CostForEvents([]schema.Event{anthropicEvent("claude-unreleased-9")})
	vc := findVendor(t, got, "anthropic")
	b := findBasis(t, vc, BasisAPIList)
	if b.USD != 0 {
		t.Fatalf("unknown model USD = %v, want 0", b.USD)
	}
}

// TestCopilotCreditsLeft_NoPlanConfigured guards the "no guessed number"
// rule: CopilotPlan unset means ok=false regardless of events.
func TestCopilotCreditsLeft_NoPlanConfigured(t *testing.T) {
	cfg := Defaults()
	_, _, ok := cfg.CopilotCreditsLeft([]schema.Event{copilotEvent("claude-sonnet-5")})
	if ok {
		t.Fatal("CopilotCreditsLeft ok = true with no CopilotPlan configured, want false")
	}
}

// TestCopilotCreditsLeft_UnknownPlan guards a CopilotPlan naming a tier the
// book does not carry (a typo, or a plan since retired): ok=false, not a
// panic or a zero-vs-unset ambiguity.
func TestCopilotCreditsLeft_UnknownPlan(t *testing.T) {
	cfg := Defaults()
	cfg.CopilotPlan = "Nonexistent"
	_, _, ok := cfg.CopilotCreditsLeft([]schema.Event{copilotEvent("claude-sonnet-5")})
	if ok {
		t.Fatal("CopilotCreditsLeft ok = true for an unknown plan, want false")
	}
}

// TestCopilotCreditsLeft_Configured guards the real calculation: the
// configured plan's MonthlyCredits minus this month's github-vendor spend
// (1470 credits, per TestCostForEvents_CopilotCreditsBasis's own fixture),
// non-github events in the same slice contributing nothing.
func TestCopilotCreditsLeft_Configured(t *testing.T) {
	cfg := Defaults()
	planName := ""
	var wantMonthly float64
	for name, p := range cfg.CopilotCredits.Plans {
		planName, wantMonthly = name, p.MonthlyCredits
		break
	}
	if planName == "" {
		t.Skip("no Copilot plan in the compiled-in book to test against")
	}
	cfg.CopilotPlan = planName

	events := []schema.Event{copilotEvent("claude-sonnet-5"), anthropicEvent("claude-sonnet-5")}
	left, plan, ok := cfg.CopilotCreditsLeft(events)
	if !ok {
		t.Fatal("CopilotCreditsLeft ok = false with a configured, known plan")
	}
	if plan.MonthlyCredits != wantMonthly {
		t.Fatalf("plan.MonthlyCredits = %v, want %v", plan.MonthlyCredits, wantMonthly)
	}
	wantLeft := wantMonthly - 1470.0
	if diff := left - wantLeft; diff > 1e-6 || diff < -1e-6 {
		t.Fatalf("left = %v, want %v", left, wantLeft)
	}
}

func hasBasis(vcs []VendorCost, vendor string, basis Basis) (BasisCost, bool) {
	for _, vc := range vcs {
		if vc.Vendor != vendor {
			continue
		}
		for _, b := range vc.Bases {
			if b.Basis == basis {
				return b, true
			}
		}
	}
	return BasisCost{}, false
}
