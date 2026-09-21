// Package dataset builds the usage payload shared by the claudecost CLI and
// the claudecost-app window: source resolution, JSONL parsing with an
// mtime/size cache, aggregation, and the JSON schema injected into the
// dashboard template.
package dataset

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"claudecost/internal/agg"
	"claudecost/internal/pricing"
	"claudecost/internal/scan"
)

// CoverageNote explains what claudecost can and cannot see. Unchanged from
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

type cacheEntry struct {
	mtime   time.Time
	size    int64
	session *scan.Session // nil means the file parsed to no usable session
}

// Cache reuses parsed sessions across calls to Collect when a transcript
// file's mtime and size are unchanged since the last call. The zero value is
// ready to use.
type Cache struct {
	entries map[string]cacheEntry

	// Sources and Files hold the folder and file lists resolved by the most
	// recent Collect call. Collect's progress callback only carries a
	// done/total file count, so a caller that wants to report the folder
	// count too (as the CLI's "Scanning N folder(s)..." line does) reads it
	// from here inside that same callback.
	Sources []string
	Files   []string

	// SlowFiles is the file list found under the slow (WSL) sources on the
	// most recent pass that actually walked them (a Collect call with
	// RefreshSlow true). A pass with RefreshSlow false reuses this list
	// instead of re-walking the WSL sources, and Collect skips the
	// mtime/size stat for any of these files that already has a cache
	// entry, since a stat over the WSL 9P file server is exactly the round
	// trip the slow cadence exists to avoid. In memory only: a restart
	// always does a full pass before the first render, so nothing here is
	// worth persisting in the gob.
	SlowFiles     []string
	SlowScannedAt time.Time

	// DroppedDuplicates is how many sessions the most recent Collect call
	// removed as cross-session duplicates: the same conversation recorded
	// under two session IDs, which a forked or resumed session produces.
	DroppedDuplicates int
}

// ---------------------------------------------------------------------------
// Disk persistence: parsed sessions survive restarts, keyed by an app
// version + pricing config fingerprint so a release or recalibration forces
// a full re-parse instead of serving stale computed costs.
// ---------------------------------------------------------------------------

// diskEntry and diskCache are the gob-serializable shapes of cacheEntry and
// Cache.entries. gob only ever sees exported fields, so these are separate
// types rather than exporting cacheEntry itself. scan.Session and
// scan.PerModel hold only exported fields, so they round-trip through gob
// correctly even though CWD and Daily carry a json:"-" tag; gob does not
// look at json tags at all.
type diskEntry struct {
	MTime   time.Time
	Size    int64
	Session *scan.Session // nil = file parsed to no usable session
}

type diskCache struct {
	Fingerprint string
	Entries     map[string]diskEntry
}

// Fingerprint identifies everything that invalidates cached parse results:
// the app version (parse logic may change between releases) and the pricing
// config (cached sessions store computed costs). WSLScan, ExtraSources and
// WSLIntervalHours are zeroed on a copy first: they are scan-location and
// cadence settings, not pricing, and hashing them in would mean editing an
// extra_sources entry throws away the entire parse cache for no reason.
func Fingerprint(appVersion string, cfg *pricing.Config) string {
	fpCfg := *cfg
	fpCfg.WSLScan = ""
	fpCfg.ExtraSources = nil
	fpCfg.WSLIntervalHours = 0
	b, _ := json.Marshal(&fpCfg)
	sum := sha256.Sum256(b)
	return appVersion + "|" + hex.EncodeToString(sum[:])
}

// Load reads a previously saved cache from path, keeping it only if its
// stored fingerprint matches fingerprint. Any problem opening or decoding
// the file, or a fingerprint mismatch, is silent: c is simply left with an
// empty cache, and the next Collect re-parses everything, same as a first
// run. There is nothing sensible to recover from a stale or corrupt cache
// file.
func (c *Cache) Load(path, fingerprint string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	var dc diskCache
	if err := gob.NewDecoder(f).Decode(&dc); err != nil {
		return
	}
	if dc.Fingerprint != fingerprint {
		return
	}

	entries := make(map[string]cacheEntry, len(dc.Entries))
	for k, v := range dc.Entries {
		entries[k] = cacheEntry{mtime: v.MTime, size: v.Size, session: v.Session}
	}
	c.entries = entries
}

// Save writes the cache to path, keeping only entries whose file path is in
// c.Files (the most recent Collect's file list), so transcripts that no
// longer exist do not accumulate in the file forever. It writes path+".tmp"
// and renames it over path, so a crash mid-write, or a concurrent reader,
// never sees a half-written cache file.
func (c *Cache) Save(path, fingerprint string) error {
	entries := make(map[string]diskEntry, len(c.Files))
	for _, f := range c.Files {
		e, ok := c.entries[f]
		if !ok {
			continue
		}
		entries[f] = diskEntry{MTime: e.mtime, Size: e.size, Session: e.session}
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(diskCache{Fingerprint: fingerprint, Entries: entries}); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
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

// Collect resolves sources (auto-detecting when opts.Sources is empty, else
// using opts.Sources/opts.SlowSources as given), lists transcript files,
// parses any that are new or changed since the last call on this Cache
// (reusing the cached session otherwise), aggregates the result and returns
// the dashboard payload for opts.Seat and opts.MonthsN.
//
// progress, if non-nil, is called once with (0, total) before parsing starts
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
	total := len(files)
	if progress != nil {
		progress(0, total)
	}
	if total == 0 {
		return Payload{}, ErrNoFiles
	}

	if c.entries == nil {
		c.entries = map[string]cacheEntry{}
	}

	// When this pass is not walking the slow tier, a slow file already in
	// the cache is trusted outright rather than stat'ed: an os.Stat over the
	// WSL 9P file server is exactly the round trip the slow cadence exists
	// to avoid. A slow file with no cache entry yet (first time it was ever
	// seen, or the cache was reset) falls through to the normal stat-and-
	// parse path below, so a first pass with RefreshSlow false still
	// produces correct numbers rather than a hole.
	var trustSlow map[string]bool
	if !opts.RefreshSlow && len(c.SlowFiles) > 0 {
		trustSlow = make(map[string]bool, len(c.SlowFiles))
		for _, f := range c.SlowFiles {
			trustSlow[f] = true
		}
	}

	// Sequential pass: stat every file and split into cache hits (reuse the
	// session already held) and misses that need parsing. A miss whose stat
	// itself failed (listed a moment ago, gone now) is parsed but never
	// cached, same as before.
	type miss struct {
		index int
		path  string
		mtime time.Time
		size  int64
		stat  bool
	}
	results := make([]*scan.Session, len(files))
	var misses []miss
	hits := 0
	for i, f := range files {
		if trustSlow[f] {
			if e, ok := c.entries[f]; ok {
				results[i] = e.session
				hits++
				continue
			}
		}
		fi, statErr := os.Stat(f)
		if statErr == nil {
			if e, ok := c.entries[f]; ok && e.mtime.Equal(fi.ModTime()) && e.size == fi.Size() {
				results[i] = e.session
				hits++
				continue
			}
			misses = append(misses, miss{index: i, path: f, mtime: fi.ModTime(), size: fi.Size(), stat: true})
			continue
		}
		misses = append(misses, miss{index: i, path: f})
	}

	// Cache hits are reported in one batch up front (they count as instantly
	// done); the workers below report the rest, one file at a time. progress
	// is not documented as concurrency-safe, so calls to it are serialized.
	var done atomic.Int64
	done.Store(int64(hits))
	var progressMu sync.Mutex
	report := func() {
		if progress == nil {
			return
		}
		progressMu.Lock()
		progress(int(done.Load()), total)
		progressMu.Unlock()
	}
	report()

	workers := min(runtime.NumCPU(), 8)
	if workers > len(misses) {
		workers = len(misses)
	}
	if workers < 1 {
		workers = 1
	}
	if len(misses) > 0 {
		jobs := make(chan miss)
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for m := range jobs {
					// results is pre-sized and each index is written by
					// exactly one goroutine, so no two workers ever touch
					// the same slot.
					results[m.index] = scan.ParseSession(m.path, cfg)
					done.Add(1)
					report()
				}
			}()
		}
		go func() {
			for _, m := range misses {
				jobs <- m
			}
			close(jobs)
		}()
		wg.Wait()
	}

	// Insert the newly parsed entries back into c.entries: single goroutine,
	// after the pool has fully drained, so the entries map itself is never
	// touched concurrently.
	for _, m := range misses {
		if !m.stat {
			continue
		}
		c.entries[m.path] = cacheEntry{mtime: m.mtime, size: m.size, session: results[m.index]}
	}

	var sessions []*scan.Session
	for _, sess := range results {
		if sess != nil {
			sessions = append(sessions, sess)
		}
	}

	// Cross-session dedup: a forked or resumed session's child transcript
	// replays the parent's records, so both parse to identical turn sets
	// under two different session IDs. dataset owns this pass, not scan,
	// because scan parses one file at a time and has no business knowing
	// about the others.
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
