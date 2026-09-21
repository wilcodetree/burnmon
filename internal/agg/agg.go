// Package agg rolls parsed sessions up into month, week and day buckets,
// ported from the build() function of claude_usage_extract.py.
package agg

import (
	"fmt"
	"math"
	"sort"
	"time"

	"claudecost/internal/scan"
)

type SurfaceAgg struct {
	Calls    int64   `json:"calls"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
	CostSub  float64 `json:"cost_sub"`
	Sessions int64   `json:"sessions"`
}

type ModelAgg struct {
	Calls   int64   `json:"calls"`
	Tokens  int64   `json:"tokens"`
	Cost    float64 `json:"cost"`
	CostSub float64 `json:"cost_sub"`
}

// ToolAgg is one connector or plugin group's tool-call footprint inside a
// bucket. Keyed by scan.ToolGroup, not the raw tool name: per-tool detail
// stays on the session rows in the payload.
type ToolAgg struct {
	Calls    int64 `json:"calls"`
	Sessions int64 `json:"sessions"`
}

type Bucket struct {
	Calls     int64                  `json:"calls"`
	Tokens    int64                  `json:"tokens"`
	Cost      float64                `json:"cost"`
	CostSub   float64                `json:"cost_sub"`
	Sessions  int64                  `json:"sessions"`
	Fresh     int64                  `json:"fresh"`
	CacheW    int64                  `json:"cache_w"`
	CacheR    int64                  `json:"cache_r"`
	Out       int64                  `json:"out"`
	BySurface map[string]*SurfaceAgg `json:"by_surface"`
	ByModel   map[string]*ModelAgg   `json:"by_model"`
	ByTool    map[string]*ToolAgg    `json:"by_tool,omitempty"`
}

func newBucket() *Bucket {
	return &Bucket{
		BySurface: map[string]*SurfaceAgg{},
		ByModel:   map[string]*ModelAgg{},
		ByTool:    map[string]*ToolAgg{},
	}
}

func get(m map[string]*Bucket, k string) *Bucket {
	b := m[k]
	if b == nil {
		b = newBucket()
		m[k] = b
	}
	return b
}

func (b *Bucket) surf(k string) *SurfaceAgg {
	s := b.BySurface[k]
	if s == nil {
		s = &SurfaceAgg{}
		b.BySurface[k] = s
	}
	return s
}

func (b *Bucket) model(k string) *ModelAgg {
	m := b.ByModel[k]
	if m == nil {
		m = &ModelAgg{}
		b.ByModel[k] = m
	}
	return m
}

func (b *Bucket) tool(k string) *ToolAgg {
	t := b.ByTool[k]
	if t == nil {
		t = &ToolAgg{}
		b.ByTool[k] = t
	}
	return t
}

func monthKey(d time.Time) string {
	return fmt.Sprintf("%04d-%02d", d.Year(), int(d.Month()))
}

func weekKey(d time.Time) string {
	y, w := d.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}

func round4(x float64) float64 {
	return math.Round(x*1e4) / 1e4
}

func parseDay(s string) (time.Time, bool) {
	d, err := time.Parse("2006-01-02", s)
	return d, err == nil
}

type target struct {
	key string
	m   map[string]*Bucket
}

// Build aggregates sessions with any activity on or after cutoff into month,
// week and day buckets, and returns the kept sessions newest first.
//
// Attribution rules, deliberately different per metric:
//   - Calls, tokens and cost go to the day (and its month and week) each call
//     actually ran, so sessions spanning midnight or a month edge split
//     correctly.
//   - Sessions on a bucket means "sessions active in this bucket": a session
//     counts once in every month, week and day it had at least one call in.
//     A July-started session still running in August is an August session
//     too. Per-bucket session counts therefore do not sum to the total.
//   - Model and tool splits and the token composition stay whole-session,
//     attributed to the session's first in-window activity day, month and
//     week: splitting them per call would need per-call model data the
//     session cache does not carry.
func Build(sessions []*scan.Session, cutoff time.Time) (months, weeks, days map[string]*Bucket, kept []*scan.Session) {
	months, weeks, days = map[string]*Bucket{}, map[string]*Bucket{}, map[string]*Bucket{}

	for _, s := range sessions {
		if len(s.Start) < 10 || len(s.End) < 10 {
			continue
		}
		if _, ok := parseDay(s.Start[:10]); !ok {
			continue
		}
		ed, ok := parseDay(s.End[:10])
		if !ok || ed.Before(cutoff) {
			continue // all activity ended before the window opened
		}

		dayKeys := make([]string, 0, len(s.Daily))
		for k := range s.Daily {
			dayKeys = append(dayKeys, k)
		}
		sort.Strings(dayKeys)

		firstDay := ""
		seenMonth := map[string]bool{}
		seenWeek := map[string]bool{}

		for _, daystr := range dayKeys {
			dv := s.Daily[daystr]
			dd, ok := parseDay(daystr)
			if !ok || dd.Before(cutoff) {
				continue
			}
			if firstDay == "" {
				firstDay = daystr
			}
			mk, wk := monthKey(dd), weekKey(dd)

			for _, t := range []target{{mk, months}, {wk, weeks}, {daystr, days}} {
				b := get(t.m, t.key)
				b.Calls += dv.Calls
				b.Tokens += dv.Tokens
				b.Cost += dv.Cost
				b.CostSub += dv.CostSub
				bs := b.surf(s.Surface)
				bs.Calls += dv.Calls
				bs.Tokens += dv.Tokens
				bs.Cost += dv.Cost
				bs.CostSub += dv.CostSub
			}

			// Activity-based session counts: once per day, once per distinct
			// month and week.
			db := get(days, daystr)
			db.Sessions++
			db.surf(s.Surface).Sessions++
			if !seenMonth[mk] {
				seenMonth[mk] = true
				mb := get(months, mk)
				mb.Sessions++
				mb.surf(s.Surface).Sessions++
			}
			if !seenWeek[wk] {
				seenWeek[wk] = true
				wb := get(weeks, wk)
				wb.Sessions++
				wb.surf(s.Surface).Sessions++
			}
		}

		if firstDay == "" {
			continue // no in-window activity at all
		}
		kept = append(kept, s)

		fd, _ := parseDay(firstDay)
		targets := []target{{monthKey(fd), months}, {weekKey(fd), weeks}, {firstDay, days}}

		// Model split, whole-session granularity.
		mkeys := make([]string, 0, len(s.Models))
		for k := range s.Models {
			mkeys = append(mkeys, k)
		}
		sort.Strings(mkeys)
		for _, t := range targets {
			b := get(t.m, t.key)
			for _, mn := range mkeys {
				mv := s.Models[mn]
				bm := b.model(mn)
				bm.Calls += mv.Calls
				bm.Tokens += mv.Tokens
				bm.Cost += mv.Cost
				bm.CostSub += mv.CostSub
			}
		}

		// Tool split, whole-session granularity, grouped by connector/plugin
		// (scan.ToolGroup) rather than raw tool name. Sessions is the count of
		// distinct sessions that used the group at least once, matching how
		// SurfaceAgg.Sessions is computed above; a session calling the same
		// group's tools several times still counts once.
		if len(s.Tools) > 0 {
			groupSeen := map[string]bool{}
			for tn := range s.Tools {
				groupSeen[scan.ToolGroup(tn)] = true
			}
			groups := make([]string, 0, len(groupSeen))
			for g := range groupSeen {
				groups = append(groups, g)
			}
			sort.Strings(groups)
			for _, t := range targets {
				b := get(t.m, t.key)
				for _, g := range groups {
					b.tool(g).Sessions++
				}
				for tn, calls := range s.Tools {
					b.tool(scan.ToolGroup(tn)).Calls += calls
				}
			}
		}

		// Token composition, attributed to the first in-window month and week.
		for _, t := range []target{{monthKey(fd), months}, {weekKey(fd), weeks}} {
			b := get(t.m, t.key)
			b.Fresh += s.Fresh
			b.CacheW += s.CacheW
			b.CacheR += s.CacheR
			b.Out += s.Out
		}
	}

	sort.Slice(kept, func(i, j int) bool { return kept[i].Start > kept[j].Start })

	for _, m := range []map[string]*Bucket{months, weeks, days} {
		for _, b := range m {
			b.Cost = round4(b.Cost)
			b.CostSub = round4(b.CostSub)
			for _, s := range b.BySurface {
				s.Cost = round4(s.Cost)
				s.CostSub = round4(s.CostSub)
			}
			for _, mv := range b.ByModel {
				mv.Cost = round4(mv.Cost)
				mv.CostSub = round4(mv.CostSub)
			}
		}
	}
	return months, weeks, days, kept
}
