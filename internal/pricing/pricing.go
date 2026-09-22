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
	"time"
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

	// ContextWindowBookDate is when the context_window table below was last
	// checked against platform.claude.com/docs/en/about-claude/models and
	// developers.openai.com/codex's model pages. VERIFY: both are published
	// standard-tier windows; Sonnet's documented 1M-token beta context is not
	// used here since it needs a separate beta header burnmon never sends.
	ContextWindowBookDate = "2026-09-22"

	// ClaudeCodeCacheBookDate is when the cache-lifetime and auto-compact
	// facts below were checked against code.claude.com/docs/en/costs and
	// code.claude.com/docs/en/model-config, closing the v0.2 spec's I2
	// VERIFY item.
	ClaudeCodeCacheBookDate = "2026-09-22"
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

// OwnerRule is one ordered rule of P6's light client map: the first rule
// whose Match matches a session's project path wins.
type OwnerRule struct {
	// Match is a path prefix, "*"-suffixed (e.g. `C:\dev\Work\*`). Matched
	// case-insensitively as a plain prefix; the trailing "*" carries no
	// other glob meaning.
	Match string `json:"match"`
	Owner string `json:"owner"`
}

type Config struct {
	FXUSDEUR       float64               `json:"fx_usd_eur"`
	CacheWriteMult float64               `json:"cache_write_mult"`
	CacheReadMult  float64               `json:"cache_read_mult"`
	FallbackFamily string                `json:"fallback_family"`
	Prices         map[string]ModelPrice `json:"prices"`
	Subscription   Subscription          `json:"subscription"`

	// Owners is P6's light client map: ordered rules matched against a
	// session's project path at ingest. Empty (the default) means one
	// owner, no owner column shown anywhere: OwnerFor returns "" for every
	// path. Non-empty means every path gets an owner: the first matching
	// rule's Owner, or "personal" when no rule matches.
	Owners []OwnerRule `json:"owners"`

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

	// ContextWindows is the Now page's context-window table, tokens per
	// exact model id (not family, unlike Prices), dated by
	// ContextWindowBookDate. A model id with no entry here is unknown: the
	// Now page's gauge shows raw tokens with no percentage for it.
	ContextWindows map[string]int64 `json:"context_window"`

	// LiveRunningWindowMinutes is how long since a session's last turn it
	// still counts as "running" on the Now page; zero or absent means the
	// compiled-in 10-minute default (the Now page features note's
	// "runningWindow" constant).
	LiveRunningWindowMinutes float64 `json:"live_running_window_minutes"`

	// ClaudeCodeCacheTTLMinutes is the prompt-cache lifetime insight's
	// re-prefill rule (v0.2 I2) uses to infer a "gap since the previous turn
	// exceeded the cache TTL" cause. Zero or absent means the compiled-in
	// 60-minute default. Verified ClaudeCodeCacheBookDate against
	// code.claude.com/docs/en/costs: on a Claude subscription seat
	// (Pro/Max/Team/Enterprise, Wilco's own setup, see Subscription above)
	// the lifetime is one hour; it drops to five minutes once the session is
	// drawing on usage credits, and is five minutes by default on a bare API
	// key or cloud provider. The 60-minute default matches the subscription
	// case; override this for an API-key or usage-credits setup.
	ClaudeCodeCacheTTLMinutes float64 `json:"claude_code_cache_ttl_minutes"`

	// ClaudeCodeAutoCompactTokens documents, but is not applied by any v0.2
	// rule, the token count at which Claude Code's own auto-compact runs.
	// Verified ClaudeCodeCacheBookDate against
	// code.claude.com/docs/en/model-config: there is no single fixed
	// percentage; Claude Code compacts when the conversation reaches the
	// model's context window, except models running a native 1M-token
	// window (Sonnet 5, the Fable models, Opus 4.7+ on the Anthropic API),
	// which compact early at about 967,000 tokens by default. burnmon's own
	// ContextWindows table above prices the 200,000-token standard tier, not
	// the 1M beta, so this number does not apply to it directly; insight's
	// compaction rule (I2) detects a compaction by its effect instead, a
	// greater-than-30%-percent context drop between consecutive turns, not
	// by comparing against this threshold.
	ClaudeCodeAutoCompactTokens int64 `json:"claude_code_auto_compact_tokens"`

	// ReprefillCacheWriteThreshold is the cache-write token count (in one
	// turn) above which insight's re-prefill rule (v0.2 I2) reports a
	// finding. Zero or absent means the compiled-in 20,000-token default
	// (the spec's "cache write in a turn exceeds 20K tokens").
	ReprefillCacheWriteThreshold int64 `json:"reprefill_cache_write_threshold"`
}

// DefaultClaudeCodeCacheTTLMinutes is the compiled-in cache-lifetime
// fallback used when ClaudeCodeCacheTTLMinutes is zero or absent: the
// subscription-seat figure verified ClaudeCodeCacheBookDate (see the field's
// comment).
const DefaultClaudeCodeCacheTTLMinutes = 60

// DefaultReprefillCacheWriteThreshold is the compiled-in re-prefill
// threshold fallback used when ReprefillCacheWriteThreshold is zero or
// absent.
const DefaultReprefillCacheWriteThreshold = 20_000

// ClaudeCodeCacheTTL returns the configured prompt-cache lifetime, falling
// back to DefaultClaudeCodeCacheTTLMinutes.
func (c *Config) ClaudeCodeCacheTTL() time.Duration {
	m := c.ClaudeCodeCacheTTLMinutes
	if m <= 0 {
		m = DefaultClaudeCodeCacheTTLMinutes
	}
	return time.Duration(m * float64(time.Minute))
}

// ReprefillThreshold returns the configured re-prefill cache-write
// threshold in tokens, falling back to DefaultReprefillCacheWriteThreshold.
func (c *Config) ReprefillThreshold() int64 {
	if c.ReprefillCacheWriteThreshold > 0 {
		return c.ReprefillCacheWriteThreshold
	}
	return DefaultReprefillCacheWriteThreshold
}

// DefaultRunningWindow is the compiled-in "running session" threshold used
// when LiveRunningWindowMinutes is zero or absent.
const DefaultRunningWindow = 10 * 60 // seconds

// RunningWindowSeconds returns the configured running-session threshold in
// seconds, falling back to DefaultRunningWindow.
func (c *Config) RunningWindowSeconds() float64 {
	if c.LiveRunningWindowMinutes > 0 {
		return c.LiveRunningWindowMinutes * 60
	}
	return DefaultRunningWindow
}

// ContextWindow returns model's context window in tokens and whether it is
// known. An unknown model (no entry in ContextWindows) returns (0, false):
// the Now page shows raw token counts with no percentage for it.
func (c *Config) ContextWindow(model string) (int64, bool) {
	w, ok := c.ContextWindows[model]
	return w, ok
}

// OwnerFor returns projectPath's owner per P6's light client map: "" when
// Owners is empty (default: one owner, no owner column shown anywhere),
// else the first rule whose Match prefixes projectPath (case-insensitive),
// else "personal" when Owners is non-empty but nothing matched.
func (c *Config) OwnerFor(projectPath string) string {
	if len(c.Owners) == 0 {
		return ""
	}
	lp := strings.ToLower(projectPath)
	for _, r := range c.Owners {
		prefix := strings.ToLower(strings.TrimSuffix(r.Match, "*"))
		if strings.HasPrefix(lp, prefix) {
			return r.Owner
		}
	}
	return "personal"
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
		// Context windows, tokens, dated ContextWindowBookDate. Anthropic
		// models: 200,000 tokens standard tier, per
		// platform.claude.com/docs/en/about-claude/models (Sonnet's 1M-token
		// beta window needs a beta header burnmon never sends, so it is not
		// used here). OpenAI Codex models: 400,000 tokens, per
		// developers.openai.com/codex's model pages for the Codex-class
		// models seen on Wilco's laptop. "codex-auto-review" has no
		// published context window (also unpriced, see OpenAIPrices) and is
		// deliberately left out, so its gauge shows tokens only.
		ContextWindows: map[string]int64{
			"claude-opus-5":             200_000,
			"claude-sonnet-5":           200_000,
			"claude-haiku-4-5-20251001": 200_000,
			"claude-fable-5-1":          200_000,
			"gpt-6-astra":               400_000,
			"gpt-5.6-sol":               400_000,
			"gpt-5.6-terra":             400_000,
			"gpt-5.6-luna":              400_000,
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
