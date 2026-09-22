// Package insight explains a session's burn instead of just counting it
// (v0.2 I1): given one session's canonical events, ordered by turn, plus its
// model's context window from the price book, it reports Findings such as a
// re-prefill or a compaction. No HTML, no store writes: every Finding is
// computed fresh from the same events the Now snapshot (internal/live) and
// the Sessions tab already load, so there is no cache to invalidate.
package insight

import (
	"fmt"
	"sort"
	"time"

	"burnmon/internal/pricing"
	"burnmon/internal/schema"
)

// Kind names one insight rule.
type Kind string

const (
	KindReprefill  Kind = "re-prefill"
	KindCompaction Kind = "compaction"
)

// Finding is one thing insight noticed about one turn of one session.
type Finding struct {
	Kind       Kind
	Turn       int // 1-based, over this session's real API-call turns only
	At         time.Time
	Evidence   map[string]float64
	Cause      string
	Confidence float64
}

// compactionDropRatio is I2's compaction threshold: a context drop greater
// than this between consecutive turns, while the session continues. Not
// configurable in v0.2, unlike the re-prefill threshold, per the spec.
const compactionDropRatio = 0.30

// isTurn reports whether e is a real API-call turn rather than the claude
// adapter's synthetic tool-only event. Mirrors internal/live's own isTurn
// and internal/dataset/fromstore.go's buildSession condition, the
// established duplicate rather than a shared export across those packages.
func isTurn(e schema.Event) bool {
	return !(e.Model == "" && e.Input == 0 && e.Output == 0)
}

// contextOf is one turn's total context sent: fresh input plus whatever was
// served from cache and whatever cache was written.
func contextOf(e schema.Event) int64 {
	c := e.Input
	if e.CacheRead != nil {
		c += *e.CacheRead
	}
	if e.CacheWrite != nil {
		c += *e.CacheWrite
	}
	return c
}

func cacheWriteOf(e schema.Event) int64 {
	if e.CacheWrite != nil {
		return *e.CacheWrite
	}
	return 0
}

// Analyze runs every v0.2 I2 rule over events, one session's worth, in any
// order (Analyze sorts by At itself), and returns every Finding in turn
// order, re-prefill and compaction interleaved. events need not be
// pre-filtered: synthetic tool-only events are dropped first, the same way
// internal/live and internal/dataset/fromstore.go already do.
func Analyze(events []schema.Event, cfg *pricing.Config) []Finding {
	turns := make([]schema.Event, 0, len(events))
	for _, e := range events {
		if isTurn(e) {
			turns = append(turns, e)
		}
	}
	sort.SliceStable(turns, func(i, j int) bool { return turns[i].At.Before(turns[j].At) })
	if len(turns) == 0 {
		return nil
	}

	var findings []Finding

	// compactionAt maps a 1-based turn number to whether I2's compaction
	// rule fired there, so the re-prefill rule below can check "compaction
	// just happened" first, per the cause order the spec fixes.
	compactionAt := map[int]bool{}
	for i := 1; i < len(turns); i++ {
		before, after := contextOf(turns[i-1]), contextOf(turns[i])
		if before <= 0 || after >= before {
			continue
		}
		drop := float64(before-after) / float64(before)
		if drop <= compactionDropRatio {
			continue
		}
		turn := i + 1
		compactionAt[turn] = true
		findings = append(findings, Finding{
			Kind: KindCompaction,
			Turn: turn,
			At:   turns[i].At,
			Evidence: map[string]float64{
				"context_before": float64(before),
				"context_after":  float64(after),
				"drop_ratio":     drop,
			},
			Cause:      "context dropped more than 30% since the previous turn",
			Confidence: 1.0,
		})
	}

	threshold := cfg.ReprefillThreshold()
	ttl := cfg.ClaudeCodeCacheTTL()
	for i, e := range turns {
		cw := cacheWriteOf(e)
		if cw <= threshold {
			continue
		}
		turn := i + 1
		cause, confidence, evidence := reprefillCause(turns, i, turn, compactionAt, ttl)
		evidence["cache_write"] = float64(cw)
		evidence["threshold"] = float64(threshold)
		findings = append(findings, Finding{
			Kind:       KindReprefill,
			Turn:       turn,
			At:         e.At,
			Evidence:   evidence,
			Cause:      cause,
			Confidence: confidence,
		})
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Turn != findings[j].Turn {
			return findings[i].Turn < findings[j].Turn
		}
		return findings[i].Kind < findings[j].Kind
	})
	return findings
}

// reprefillCause implements I2's fixed inference order: compaction just
// happened, model changed since the previous turn, gap since the previous
// turn exceeded the cache TTL, first turn after resume, otherwise unknown.
// The first three all need a previous turn to compare against; turn is
// 1-based, i is turns' 0-based index of the same event.
func reprefillCause(turns []schema.Event, i, turn int, compactionAt map[int]bool, ttl time.Duration) (cause string, confidence float64, evidence map[string]float64) {
	evidence = map[string]float64{}
	if i == 0 {
		// No previous turn in this session's events to compare against.
		// insight has no explicit "this session resumed" signal from the
		// store, so a large cache write on the very first turn we see is
		// read as a resumed session picking back up, per the spec's fixed
		// cause list; a genuinely new session's first turn also always
		// writes its whole prompt to cache, so this cause is a
		// low-confidence best guess, not a hard fact.
		return "first turn after resume", 0.4, evidence
	}
	if compactionAt[turn] || compactionAt[turn-1] {
		return "compaction just happened", 0.9, evidence
	}
	prev, cur := turns[i-1], turns[i]
	if prev.Model != "" && cur.Model != "" && prev.Model != cur.Model {
		evidence["prev_model_changed"] = 1
		return "model changed since the previous turn", 0.8, evidence
	}
	if !prev.At.IsZero() && !cur.At.IsZero() {
		gap := cur.At.Sub(prev.At)
		evidence["gap_seconds"] = gap.Seconds()
		evidence["ttl_seconds"] = ttl.Seconds()
		if gap > ttl {
			return fmt.Sprintf("gap %d min, exceeds %d min cache TTL", int(gap.Minutes()), int(ttl.Minutes())), 0.7, evidence
		}
	}
	return "unknown", 0.3, evidence
}
