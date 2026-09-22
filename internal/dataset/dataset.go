// Package dataset builds the usage payload shared by the burnmon CLI and
// the burnmon window: source resolution, JSONL parsing with an
// mtime/size cache, aggregation, and the JSON schema injected into the
// dashboard template.
package dataset

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"burnmon/internal/adapter"
	"burnmon/internal/adapter/claude"
	"burnmon/internal/agg"
	"burnmon/internal/pricing"
	"burnmon/internal/scan"
	"burnmon/internal/store"
)

// CoverageNote explains what burnmon can and cannot see. Unchanged from
// v0.1.0.
const CoverageNote = "Local Cowork and Claude Code sessions only, from this machine. Cowork " +
	"sessions that run in Anthropic's cloud, claude.ai browser chats and mobile chats keep " +
	"no local transcript and are deliberately not collected: the only source for those " +
	"would be an organisation-wide export (Compliance API / OpenTelemetry)."

// Distinct errors Collect can return. Callers map each to their own message
// and exit code.
var (
	ErrNoSources  = errors.New("no source folders found")
	ErrNoFiles    = errors.New("no transcript files found")
	ErrNoSessions = errors.New("no sessions inside the reporting window")
)

// SeatError reports an unknown seat tier, with the valid options attached so
// callers can print them without recomputing the list.
type SeatError struct {
	Seat  string
	Valid []string
}

func (e *SeatError) Error() string {
	return fmt.Sprintf("unknown seat tier %q, valid: %s", e.Seat, strings.Join(e.Valid, ", "))
}

func validateSeat(cfg *pricing.Config, seat string) error {
	if _, ok := cfg.Subscription.SeatPriceUSD[seat]; ok {
		return nil
	}
	var valid []string
	for k := range cfg.Subscription.SeatPriceUSD {
		valid = append(valid, k)
	}
	sort.Strings(valid)
	return &SeatError{Seat: seat, Valid: valid}
}

// ---------------------------------------------------------------------------
// Payload, schema 1 of claude_usage_extract.py
// ---------------------------------------------------------------------------

type priceOut struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cache_write"`
	CacheRead  float64 `json:"cache_read"`
}

type subOut struct {
	MonthlySubscriptionUSD float64            `json:"monthly_subscription_usd"`
	MonthlySubscriptionEUR float64            `json:"monthly_subscription_eur"`
	SeatsPurchased         int                `json:"seats_purchased"`
	UsageCreditsBalanceEUR float64            `json:"usage_credits_balance_eur"`
	CompanyConsumptionUSD  float64            `json:"company_consumption_usd"`
	Seats                  map[string]int     `json:"seats"`
	SeatPriceUSD           map[string]float64 `json:"seat_price_usd"`
	OutputCostFactor       float64            `json:"output_cost_factor"`
	CalibratedOn           string             `json:"calibrated_on"`
	Window                 string             `json:"window"`
	YourSeat               string             `json:"your_seat"`
	YourSeatPriceUSD       float64            `json:"your_seat_price_usd"`
}

type totalsOut struct {
	Sessions int     `json:"sessions"`
	Calls    int64   `json:"calls"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
	CostSub  float64 `json:"cost_sub"`
}

// Payload is the full dataset injected into the dashboard template.
type Payload struct {
	Schema           int                    `json:"schema"`
	GeneratedAt      string                 `json:"generated_at"`
	WindowFrom       string                 `json:"window_from"`
	WindowTo         string                 `json:"window_to"`
	Prices           map[string]priceOut    `json:"prices_usd_per_mtok"`
	RefreshTaskID    string                 `json:"refresh_task_id"`
	Subscription     subOut                 `json:"subscription"`
	SurfaceLabels    map[string]string      `json:"surface_labels"`
	LongSessionCalls int                    `json:"long_session_calls"`
	Totals           totalsOut              `json:"totals"`
	Months           map[string]*agg.Bucket `json:"months"`
	Weeks            map[string]*agg.Bucket `json:"weeks"`
	Days             map[string]*agg.Bucket `json:"days"`
	Sessions         []*scan.Session        `json:"sessions"`
	CoverageNote     string                 `json:"coverage_note"`
}

func round4(x float64) float64 { return math.Round(x*1e4) / 1e4 }

func monthStart(t time.Time, back int) time.Time {
	y, m := t.Year(), int(t.Month())-back
	for m <= 0 {
		m += 12
		y--
	}
	return time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
}

// BuildPayload assembles the schema-1 payload from already-aggregated data.
func BuildPayload(cfg *pricing.Config, seat string, cutoff, today time.Time,
	months, weeks, days map[string]*agg.Bucket, kept []*scan.Session) Payload {

	prices := map[string]priceOut{}
	for _, fam := range cfg.Families() {
		p := cfg.Prices[fam]
		prices[p.Label] = priceOut{
			Input:      p.In,
			Output:     p.Out,
			CacheWrite: round4(p.In * cfg.CacheWriteMult),
			CacheRead:  round4(p.In * cfg.CacheReadMult),
		}
	}

	t := totalsOut{Sessions: len(kept)}
	for _, m := range months {
		t.Calls += m.Calls
		t.Tokens += m.Tokens
		t.Cost += m.Cost
		t.CostSub += m.CostSub
	}
	t.Cost = round4(t.Cost)
	t.CostSub = round4(t.CostSub)

	sub := cfg.Subscription
	return Payload{
		Schema:      2, // v0.5.0: added by_tool to agg.Bucket
		GeneratedAt: time.Now().UTC().Format("2006-01-02T15:04:05") + "+00:00",
		WindowFrom:  cutoff.Format("2006-01-02"),
		WindowTo:    today.Format("2006-01-02"),
		Prices:      prices,
		Subscription: subOut{
			MonthlySubscriptionUSD: sub.MonthlySubscriptionUSD,
			MonthlySubscriptionEUR: sub.MonthlySubscriptionEUR,
			SeatsPurchased:         sub.SeatsPurchased,
			UsageCreditsBalanceEUR: sub.UsageCreditsBalanceEUR,
			CompanyConsumptionUSD:  sub.CompanyConsumptionUSD,
			Seats:                  sub.Seats,
			SeatPriceUSD:           sub.SeatPriceUSD,
			OutputCostFactor:       sub.OutputCostFactor,
			CalibratedOn:           sub.CalibratedOn,
			Window:                 sub.Window,
			YourSeat:               seat,
			YourSeatPriceUSD:       sub.SeatPriceUSD[seat],
		},
		SurfaceLabels:    scan.SurfaceLabel,
		LongSessionCalls: scan.LongSessionCalls,
		Totals:           t,
		Months:           months,
		Weeks:            weeks,
		Days:             days,
		Sessions:         kept,
		CoverageNote:     CoverageNote,
	}
}

// ---------------------------------------------------------------------------
// Cache: mtime/size-aware parsing shared by the CLI and the app window
// ---------------------------------------------------------------------------

// Cache resolves sources, ingests new transcript bytes into the store, and
// builds the dashboard Payload from what the store now holds. The name is
// kept from v0.0.1's in-memory/gob cache for minimal call-site churn in
// cmd/burnmon and cmd/burnmon-cli; what it actually caches now is durable,
// in Store, not in this struct.
type Cache struct {
	Store *store.Store

	// Sources, Files, SlowFiles, SlowScannedAt, DroppedDuplicates: same
	// meaning as v0.0.1, read by cmd/burnmon for its stamp/log lines.
	Sources           []string
	Files             []string
	SlowFiles         []string
	SlowScannedAt     time.Time
	DroppedDuplicates int
}

// dedupSessions removes the same conversation when it was recorded under
// two session IDs. The key is five fields already on scan.Session: start,
// end, call count, output tokens and cache-read tokens. Two genuinely
// different conversations would need identical start and end timestamps
// to the millisecond plus identical token counts to collide, so this is
// safe without any requestId bookkeeping across files. On a collision the
// session whose SessionID sorts first is kept, so the choice is
// deterministic across runs and independent of file mtime.
func dedupSessions(sessions []*scan.Session) ([]*scan.Session, int) {
	best := map[string]*scan.Session{}
	for _, s := range sessions {
		key := s.Start + "|" + s.End + "|" +
			strconv.FormatInt(s.Calls, 10) + "|" +
			strconv.FormatInt(s.Out, 10) + "|" +
			strconv.FormatInt(s.CacheR, 10)
		if prev, ok := best[key]; !ok || s.SessionID < prev.SessionID {
			best[key] = s
		}
	}
	kept := make([]*scan.Session, 0, len(best))
	for _, s := range best {
		kept = append(kept, s)
	}
	return kept, len(sessions) - len(kept)
}

// ---------------------------------------------------------------------------
// Source resolution: auto-detect vs. explicit, WSL's slow tier kept separate
// ---------------------------------------------------------------------------

// CollectOpts groups Collect's per-call parameters. Sources and SlowSources
// are what the caller already knows, not what Collect should go find:
// resolving WSL sources can touch the filesystem over the 9P file server and
// even cold-start a stopped distro, which must never happen off the
// caller's own decided cadence (see RefreshSlow, and "Two refresh cadences"
// in docs/2026-08-17_wsl-source-detection-design.md).
type CollectOpts struct {
	Seat    string
	MonthsN int

	// Sources is the full resolved source list. Empty means auto-detect:
	// Collect calls scan.DefaultSourcesWithOptions itself, gated by
	// RefreshSlow so a fast-tier pass never touches WSL.
	Sources []string

	// SlowSources is the subset of Sources that is slow to walk (WSL over
	// the 9P file server). May be empty. Only consulted when Sources is
	// non-empty; the auto-detect path (Sources empty) computes its own
	// split instead.
	SlowSources []string

	// RefreshSlow requests a full pass: resolve (when auto-detecting) and
	// walk the slow sources this time, rather than reusing the file list
	// from the last pass that did.
	RefreshSlow bool

	// ForceFull re-reads every file from offset 0 even if its stored cursor
	// says it is fully read, the store-backed equivalent of v0.0.1's
	// -no-cache.
	ForceFull bool
}

// resolveSources implements the source-list half of "Two refresh cadences".
// On a full pass it is free to auto-detect, including WSL. On a fast-tier
// pass, an auto-detect must never call anything that can touch a WSL
// distro, so it resolves native sources only and leaves the slow tier to
// whatever Cache.SlowFiles already holds from the last full pass.
// cfg.ExtraSources is appended to the fast tier in every case, then the
// caller dedupes.
func resolveSources(cfg *pricing.Config, opts CollectOpts) (fast, slow []string) {
	switch {
	case len(opts.Sources) > 0:
		slow = opts.SlowSources
		fast = subtractCaseInsensitive(opts.Sources, slow)
	case opts.RefreshSlow:
		native := scan.DefaultSourcesWithOptions(false)
		full := scan.DefaultSourcesWithOptions(cfg.WSLScan != "off")
		fast = native
		slow = subtractCaseInsensitive(full, native)
	default:
		fast = scan.DefaultSourcesWithOptions(false)
	}
	fast = append(fast, cfg.ExtraSources...)
	return fast, slow
}

// subtractCaseInsensitive returns the entries of all that are not present in
// remove, comparing case-insensitively (Windows paths), preserving all's
// order.
func subtractCaseInsensitive(all, remove []string) []string {
	if len(remove) == 0 {
		return append([]string(nil), all...)
	}
	skip := make(map[string]bool, len(remove))
	for _, r := range remove {
		skip[strings.ToLower(r)] = true
	}
	var out []string
	for _, a := range all {
		if !skip[strings.ToLower(a)] {
			out = append(out, a)
		}
	}
	return out
}

// dedupeCaseInsensitive removes case-insensitive duplicate paths, keeping
// the first occurrence and otherwise preserving order, so a source and an
// extra_sources entry pointing at the same folder with different casing
// (both valid on Windows) are not walked twice.
func dedupeCaseInsensitive(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		k := strings.ToLower(s)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}

// filesUnderAny reports which of files sit under one of the given source
// roots, used to split the slow (WSL) tier's file list out of a full
// FindJSONL pass so it can be reused, unwalked, on the next fast-tier pass.
func filesUnderAny(files, roots []string) []string {
	if len(roots) == 0 {
		return nil
	}
	var out []string
	for _, f := range files {
		lf := strings.ToLower(f)
		for _, r := range roots {
			lr := strings.ToLower(r)
			if lf == lr || strings.HasPrefix(lf, lr+string(os.PathSeparator)) || strings.HasPrefix(lf, lr+"/") {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// Registered adapters. v0.1 Step 1 wires only Claude; Step 2 adds Codex
// alongside it without touching this loop's shape.
var adapters = []adapter.Adapter{claude.Adapter{}}

func adapterFor(name string) adapter.Adapter {
	for _, a := range adapters {
		if a.Name() == name {
			return a
		}
	}
	return nil
}

// ingest reads every new byte of every file in files through the claude
// adapter, upserting Events and advancing that file's cursor in c.Store. A
// file whose stored cursor already covers its current size and mtime is
// skipped without opening it. trustSlow files skip the stat entirely (an
// os.Stat over the WSL 9P file server is exactly the round trip the slow
// cadence exists to avoid), trusting the store's cursor outright the same
// way v0.0.1 trusted its in-memory cache entry outright for those files.
func (c *Cache) ingest(files []string, trustSlow map[string]bool, forceFull bool, progress func(done, total int)) error {
	total := len(files)
	if progress != nil {
		progress(0, total)
	}
	a := adapterFor("claude")
	for i, f := range files {
		offset, mtime, size, ok, err := c.Store.Cursor(f)
		if err != nil {
			return fmt.Errorf("dataset: cursor for %s: %w", f, err)
		}
		if forceFull {
			ok = false
		}
		if ok && trustSlow[f] {
			if progress != nil {
				progress(i+1, total)
			}
			continue
		}
		fi, statErr := os.Stat(f)
		if statErr != nil {
			if progress != nil {
				progress(i+1, total)
			}
			continue // listed a moment ago, gone now; same as v0.0.1's parse-miss-and-drop
		}
		if ok && !forceFull && mtime.Equal(fi.ModTime()) && size == fi.Size() && offset == fi.Size() {
			if progress != nil {
				progress(i+1, total)
			}
			continue // fully read already, nothing new
		}
		startOffset := offset
		if !ok || forceFull || !mtime.Equal(fi.ModTime()) && fi.Size() < size {
			// File shrank or its mtime moved backward relative to what the
			// cursor recorded: it was rewritten, not appended to. Re-read
			// from the start rather than trust a now-meaningless offset.
			startOffset = 0
		}
		events, newOffset, err := a.Parse(f, startOffset)
		if err != nil {
			if progress != nil {
				progress(i+1, total)
			}
			continue
		}
		if len(events) > 0 {
			if err := c.Store.UpsertEvents(events); err != nil {
				return fmt.Errorf("dataset: upsert events for %s: %w", f, err)
			}
		}
		if err := c.Store.SetCursor(f, newOffset, fi.ModTime(), fi.Size()); err != nil {
			return fmt.Errorf("dataset: set cursor for %s: %w", f, err)
		}
		if progress != nil {
			progress(i+1, total)
		}
	}
	return nil
}

// Collect resolves sources (auto-detecting when opts.Sources is empty, else
// using opts.Sources/opts.SlowSources as given), lists transcript files,
// ingests any bytes that are new or changed since the last call on this
// Cache's store, rebuilds sessions from the store's Events, aggregates the
// result and returns the dashboard payload for opts.Seat and opts.MonthsN.
//
// progress, if non-nil, is called once with (0, total) before ingest starts
// and again after every file with (doneSoFar, total).
func (c *Cache) Collect(cfg *pricing.Config, opts CollectOpts, progress func(done, total int)) (Payload, error) {
	if err := validateSeat(cfg, opts.Seat); err != nil {
		return Payload{}, err
	}

	fast, slow := resolveSources(cfg, opts)
	fast = dedupeCaseInsensitive(fast)
	slow = dedupeCaseInsensitive(slow)
	all := dedupeCaseInsensitive(append(append([]string{}, fast...), slow...))
	c.Sources = all
	if len(all) == 0 {
		return Payload{}, ErrNoSources
	}

	var files []string
	if opts.RefreshSlow {
		files = scan.FindJSONL(all)
		c.SlowFiles = filesUnderAny(files, slow)
		c.SlowScannedAt = time.Now()
	} else {
		files = scan.FindJSONL(fast)
		files = append(files, c.SlowFiles...)
	}
	c.Files = files
	if len(files) == 0 {
		return Payload{}, ErrNoFiles
	}

	var trustSlow map[string]bool
	if !opts.RefreshSlow && len(c.SlowFiles) > 0 {
		trustSlow = make(map[string]bool, len(c.SlowFiles))
		for _, f := range c.SlowFiles {
			trustSlow[f] = true
		}
	}
	if err := c.ingest(files, trustSlow, opts.ForceFull, progress); err != nil {
		return Payload{}, err
	}

	events, err := c.Store.AllEvents()
	if err != nil {
		return Payload{}, err
	}
	sessions := SessionsFromEvents(events, cfg)
	sessions, c.DroppedDuplicates = dedupSessions(sessions)
	log.Printf("dropped %d duplicate session(s)", c.DroppedDuplicates)

	today := time.Now()
	cutoff := monthStart(today, max(0, opts.MonthsN-1))
	months, weeks, days, kept := agg.Build(sessions, cutoff)
	if len(kept) == 0 {
		return Payload{}, ErrNoSessions
	}
	return BuildPayload(cfg, opts.Seat, cutoff, today, months, weeks, days, kept), nil
}
