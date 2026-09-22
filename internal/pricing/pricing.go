// Package pricing holds the price list, the subscription calibration, and the
// two cost models ported from claude_usage_extract.py. Defaults are compiled
// in; a burnmon.json next to the exe overrides any subset of them, so a
// quarterly recalibration means distributing one small file, not a rebuild.
package pricing

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ModelPrice is USD per million tokens at published API list rates.
type ModelPrice struct {
	Label string  `json:"label"`
	In    float64 `json:"in"`
	Out   float64 `json:"out"`
}

// OpenAIModelPrice is USD per million tokens for one Codex model id, keyed
// by exact model string rather than a family (unlike Claude's ModelPrice,
// OpenAI model ids are not aliased into families here). CachedIn is an
// explicit rate rather than a multiplier off In: every model id priced in
// Defaults happens to cache at 10% of its input rate, but the rate card is
// not guaranteed to keep that ratio for a model added later.
type OpenAIModelPrice struct {
	Label    string  `json:"label"`
	In       float64 `json:"in"`
	CachedIn float64 `json:"cached_in"`
	Out      float64 `json:"out"`
}

// AnthropicPriceBookDate and OpenAIPriceBookDate are the dates the
// compiled-in list prices below were last checked against their source
// pages, printed by `burnmon-cli.exe price-check`.
const (
	AnthropicPriceBookDate = "2026-09-22"
	OpenAIPriceBookDate    = "2026-09-22"
)

// Subscription is the calibration that turns token counts into a share of the
// real monthly invoice. Source: the actual invoice, not seats times list
// price. Recalibrate quarterly or when seats or prices change; see the
// comments in claude_usage_extract.py for the procedure.
type Subscription struct {
	MonthlySubscriptionEUR    float64            `json:"monthly_subscription_eur"`
	MonthlySubscriptionUSD    float64            `json:"monthly_subscription_usd"`
	SeatsPurchased            int                `json:"seats_purchased"`
	Seats                     map[string]int     `json:"seats"`
	SeatPriceUSD              map[string]float64 `json:"seat_price_usd"`
	UsageCreditsBalanceEUR    float64            `json:"usage_credits_balance_eur"`
	UsageCreditsSpentEUR      float64            `json:"usage_credits_spent_eur"`
	UsageCreditsMonthlyCapEUR float64            `json:"usage_credits_monthly_cap_eur"`
	CompanyConsumptionUSD     float64            `json:"company_consumption_usd"`
	OutputCostFactor          float64            `json:"output_cost_factor"`
	CalibratedOn              string             `json:"calibrated_on"`
	Window                    string             `json:"window"`

	// YourSeat is the default "Your seat" tier, a key into SeatPriceUSD
	// (e.g. "Standard" or "Premium"). Empty means no override: both binaries
	// fall back to their -seat flag default ("Standard"). Set this when
	// everyone who will run this burnmon.json shares one seat tier, so
	// they never have to touch Settings or pass -seat themselves. The -seat
	// flag, when explicitly passed, still wins over this.
	YourSeat string `json:"your_seat,omitempty"`
}

type Config struct {
	FXUSDEUR       float64               `json:"fx_usd_eur"`
	CacheWriteMult float64               `json:"cache_write_mult"`
	CacheReadMult  float64               `json:"cache_read_mult"`
	FallbackFamily string                `json:"fallback_family"`
	Prices         map[string]ModelPrice `json:"prices"`
	Subscription   Subscription          `json:"subscription"`

	// OpenAIPrices is the Codex price book, keyed by exact model id (e.g.
	// "gpt-5.6-terra"), populated with only the ids seen on Wilco's laptop
	// (SESSION_LOG.md, v0.1 Step 2). A model id with no entry here is
	// unpriced: OpenAICallCostUSD below returns 0 and unpriced=true so the
	// caller can count it under "unpriced" rather than guess a price.
	OpenAIPrices map[string]OpenAIModelPrice `json:"openai_prices"`

	// WSLScan controls whether source detection is allowed to probe WSL
	// distributions for Claude Code transcripts. "auto" (the default, same
	// as an empty string) scans; "off" skips it entirely, and is the first
	// thing to flip when diagnosing a slow scan. Any other value is a config
	// error caught at load time.
	WSLScan string `json:"wsl_scan"`

	// ExtraSources is appended to whatever auto-detection (or an explicit
	// -source override) already found, deduplicated afterwards. Unlike
	// -source, which replaces auto-detection, this is additive: the escape
	// hatch for a distro registered under a different Windows account, a
	// CLAUDE_CONFIG_DIR set somewhere detection does not look (a systemd
	// unit, a wrapper script, an IDE launch config), or a path on a network
	// share.
	ExtraSources []string `json:"extra_sources"`

	// WSLIntervalHours sets the slow (WSL) refresh cadence in the app, in
	// hours; zero or absent means the compiled-in 4-hour default. The app's
	// -wsl-interval flag wins over this when both are present.
	WSLIntervalHours float64 `json:"wsl_interval_hours"`
}

// Defaults mirrors the PRICES and SUBSCRIPTION blocks of
// claude_usage_extract.py. Keep the two in sync when either changes.
func Defaults() Config {
	return Config{
		FXUSDEUR:       0.92,
		CacheWriteMult: 1.25,
		CacheReadMult:  0.10,
		FallbackFamily: "sonnet",
		Prices: map[string]ModelPrice{
			"opus":   {Label: "Opus", In: 5.0, Out: 25.0},
			"sonnet": {Label: "Sonnet", In: 3.0, Out: 15.0},
			"haiku":  {Label: "Haiku", In: 1.0, Out: 5.0},
			"fable":  {Label: "Fable", In: 10.0, Out: 50.0},
		},
		// OpenAI list prices, USD per million tokens, checked 2026-09-22
		// against developers.openai.com/api/docs/pricing (see
		// SESSION_LOG.md, v0.1 Step 2). Only the model ids actually seen in
		// Wilco's own ~/.codex/sessions trail: "codex-auto-review", also
		// seen there, has no published per-token rate and is deliberately
		// left out, so it prices as 0/unpriced.
		OpenAIPrices: map[string]OpenAIModelPrice{
			"gpt-6-astra":   {Label: "GPT-6 Astra", In: 10.00, CachedIn: 1.00, Out: 50.00},
			"gpt-5.6-sol":   {Label: "GPT-5.6 Sol", In: 4.00, CachedIn: 0.40, Out: 20.00},
			"gpt-5.6-terra": {Label: "GPT-5.6 Terra", In: 2.00, CachedIn: 0.20, Out: 12.00},
			"gpt-5.6-luna":  {Label: "GPT-5.6 Luna", In: 0.20, CachedIn: 0.02, Out: 1.20},
		},
		Subscription: Subscription{
			// Illustrative example calibration, not a real invoice. Drop a
			// burnmon.json next to the exe with your own numbers; see
			// claudecost.example.json and the README section on calibration.
			MonthlySubscriptionEUR:    2000.00,
			MonthlySubscriptionUSD:    2160.00,
			SeatsPurchased:            50,
			Seats:                     map[string]int{"Standard": 45, "Premium": 5},
			SeatPriceUSD:              map[string]float64{"Standard": 20.00, "Premium": 100.00},
			UsageCreditsBalanceEUR:    0,
			UsageCreditsSpentEUR:      0,
			UsageCreditsMonthlyCapEUR: 0,
			CompanyConsumptionUSD:     1000.00,
			OutputCostFactor:          1.8085,
			CalibratedOn:              "example",
			Window:                    "example",
		},
		// WSLScan, ExtraSources and WSLIntervalHours are left at their zero
		// values here: "" means auto-scan, nil means nothing extra, 0 means
		// the compiled-in 4-hour WSL cadence, all identical to today's
		// behaviour for anyone who has never heard of WSL detection.
	}
}

// validWSLScan reports whether v is an accepted value for wsl_scan: "" and
// "auto" both mean the default (scan), "off" disables it.
func validWSLScan(v string) bool {
	return v == "" || v == "auto" || v == "off"
}

// Load returns the defaults with any fields present in the JSON file at path
// merged over them. An empty path returns plain defaults.
func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	if !validWSLScan(cfg.WSLScan) {
		return cfg, fmt.Errorf("%s: wsl_scan %q is not valid, use \"auto\" or \"off\"", path, cfg.WSLScan)
	}
	return cfg, nil
}

// Families returns model families in a fixed order: the known four first,
// then anything extra from a config override, sorted.
func (c *Config) Families() []string {
	known := []string{"opus", "sonnet", "haiku", "fable"}
	seen := map[string]bool{}
	var out []string
	for _, f := range known {
		if _, ok := c.Prices[f]; ok {
			out = append(out, f)
			seen[f] = true
		}
	}
	var extra []string
	for f := range c.Prices {
		if !seen[f] {
			extra = append(extra, f)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}

// ModelFamily maps a model string like "claude-sonnet-5" to a price family.
func (c *Config) ModelFamily(model string) string {
	m := strings.ToLower(model)
	for _, fam := range c.Families() {
		if strings.Contains(m, fam) {
			return fam
		}
	}
	return c.FallbackFamily
}

func (c *Config) price(fam string) ModelPrice {
	if p, ok := c.Prices[fam]; ok {
		return p
	}
	return c.Prices[c.FallbackFamily]
}

// Label returns the display label for a family.
func (c *Config) Label(fam string) string {
	return c.price(fam).Label
}

// CallCostUSD is the full API list price of one call: every token, cache included.
func (c *Config) CallCostUSD(fam string, fresh, cacheW, cacheR, out int64) float64 {
	p := c.price(fam)
	return (float64(fresh)*p.In +
		float64(cacheW)*p.In*c.CacheWriteMult +
		float64(cacheR)*p.In*c.CacheReadMult +
		float64(out)*p.Out) / 1e6
}

// CallCostSubUSD is the share of the subscription this call accounts for.
// Output-driven, because that is how Anthropic's consumption meter behaves:
// cache traffic is not charged.
func (c *Config) CallCostSubUSD(fam string, fresh, out int64) float64 {
	p := c.price(fam)
	return (float64(out)*p.Out + float64(fresh)*p.In) / 1e6 * c.Subscription.OutputCostFactor
}

// OpenAILabel returns model's display label, or model itself when it has no
// price-book entry (still useful as a per-model grouping key).
func (c *Config) OpenAILabel(model string) string {
	if p, ok := c.OpenAIPrices[model]; ok {
		return p.Label
	}
	return model
}

// OpenAICallCostUSD is the full API list price of one Codex turn. unpriced
// is true when model has no price-book entry, in which case cost is always
// 0 and the caller is expected to count the turn's tokens under "unpriced"
// rather than guess a rate for it.
func (c *Config) OpenAICallCostUSD(model string, fresh, cacheRead, out int64) (cost float64, unpriced bool) {
	p, ok := c.OpenAIPrices[model]
	if !ok {
		return 0, true
	}
	return (float64(fresh)*p.In + float64(cacheRead)*p.CachedIn + float64(out)*p.Out) / 1e6, false
}
