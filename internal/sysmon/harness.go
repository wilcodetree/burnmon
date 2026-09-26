// Package sysmon is BurnMon Dev's system-side collector: process-to-harness
// mapping, the live sampler, and the sample store (burnmon-dev.db), kept
// separate from BurnMon's own token/cost store (see 04_assets
// 2026-09-24_burnmon_dev_design.md, sections 4 and 6). The sampling
// approach in sample_windows.go is copied from perfadvisor's own
// internal/collect/*_windows.go and internal/tui/sample.go (perfadvisor
// main 2ed8046, Wilco's own repo, no licence issue); the PDH, GPU and
// pressure collectors those files also have are not ported yet and arrive
// in phase 3 alongside the system zone.
package sysmon

import "strings"

// Harness is one of the AI coding tools BurnMon Dev's process table groups
// by, or one of the non-harness buckets (WSL, itself, unclassified).
type Harness string

const (
	HarnessClaude        Harness = "claude"
	HarnessClaudeDesktop Harness = "claude-desktop"
	HarnessCodex         Harness = "codex"
	HarnessCopilotCLI    Harness = "copilot-cli"
	HarnessCopilotVSCode Harness = "copilot-vscode"
	HarnessHermes        Harness = "hermes"
	HarnessWSL           Harness = "wsl"
	HarnessSelf          Harness = "burnmon-dev"
	// HarnessSelfWebview (WS2 item 4, "honest self row") is burnmon-dev.exe's
	// own WebView2 host process tree (msedgewebview2.exe, split by parent
	// chain from any other app's own WebView2 processes below), kept out
	// of HarnessSelf so the process-groups panel reports the Go process and
	// its WebView2 renderer separately rather than one summed "BurnMon Dev"
	// row that silently mixed both (Wilco's own "BurnMon Dev 47%"
	// observation, phase 5b, could not otherwise be read as meaning either
	// one specifically).
	HarnessSelfWebview      Harness = "burnmon-dev-webview2"
	HarnessNodeUnclassified Harness = "node"
	HarnessOther            Harness = "other"
)

// Proc is the minimal process fact Classify needs. Deliberately independent
// of gopsutil's own process type so this file's tests exercise every rule
// without a real process tree; the sampler adapts a gopsutil process into
// one of these.
type Proc struct {
	PID     int32
	PPID    int32
	Name    string
	Cmdline string
}

// Classify maps one process to the harness it belongs to, design doc
// section 4's rule table: command-line substring first, then exact process
// name, then a parent-chain lookup, else HarnessOther.
//
// copilotVSCodeConfigured is true only when burnmon.json's
// copilot_vscode_otel_file is set: that is the only signal that ties a VS
// Code process to Copilot at all (see the design doc), so a plain VS Code
// window is never misclassified as Copilot when OTel is off.
//
// parentHarness looks up an already-classified ancestor by pid; pass nil to
// skip parent-chain inheritance (e.g. the caller has not classified the
// tree yet). It returns ok=false for a pid it has no classification for.
func Classify(p Proc, copilotVSCodeConfigured bool, parentHarness func(pid int32) (Harness, bool)) Harness {
	// A scoped npm package's on-disk path mirrors its name with OS path
	// separators ("node_modules\@anthropic-ai\claude-code\cli.js" on
	// Windows), so normalize to "/" before matching the package name as a
	// plain substring instead of only ever matching a Unix-style cmdline.
	cmd := strings.ReplaceAll(strings.ToLower(p.Cmdline), `\`, "/")
	name := strings.ToLower(p.Name)

	switch {
	case strings.Contains(cmd, "@anthropic-ai/claude-code"):
		return HarnessClaude
	case strings.Contains(cmd, "@openai/codex"):
		return HarnessCodex
	case strings.Contains(cmd, "github-copilot-cli"):
		return HarnessCopilotCLI
	case copilotVSCodeConfigured && isVSCodeName(name) && strings.Contains(cmd, "copilot"):
		return HarnessCopilotVSCode
	case strings.Contains(cmd, "hermes"):
		return HarnessHermes
	case strings.Contains(cmd, "claude desktop") || strings.Contains(cmd, "anthropic-cowork"):
		return HarnessClaudeDesktop
	}

	switch {
	// Claude Desktop's Electron shell and Claude Code's own CLI binary
	// share a name once lower-cased ("claude.exe" on a case-insensitive
	// filesystem); the original-case check catches the capitalized
	// Electron shell name before the generic lower-case check below does,
	// per the design doc's own noted ambiguity.
	case p.Name == "Claude.exe" || p.Name == "Claude":
		return HarnessClaudeDesktop
	case name == "claude.exe" || name == "claude":
		return HarnessClaude
	case name == "codex.exe" || name == "codex":
		return HarnessCodex
	case name == "copilot.exe" || name == "copilot" || name == "gh-copilot":
		return HarnessCopilotCLI
	case name == "hermes" || name == "hermes.exe":
		return HarnessHermes
	case name == "vmmem" || name == "vmmemwsl":
		return HarnessWSL
	case name == "burnmon-dev.exe" || name == "burnmon-dev":
		return HarnessSelf
	}

	// A WebView2 host process (the browser process, plus its own renderer/
	// GPU/utility children, all also named msedgewebview2.exe) descending
	// from burnmon-dev.exe itself gets its own bucket rather than the
	// generic "inherit whatever the parent already resolved to" fallback
	// below, which would otherwise report it as plain HarnessSelf, mixing
	// the Go process and its WebView2 host into one indistinguishable sum.
	// A msedgewebview2.exe belonging to some other app (a real Edge
	// browser, another WebView2-hosted tool) is deliberately NOT special
	// -cased here: parentHarness(p.PPID) for it resolves to that other
	// app's own harness (or HarnessOther), never HarnessSelfWebview, since
	// the check below only fires when the nearest already-classified
	// ancestor is this app's own Go process or its own WebView2 tree.
	if (name == "msedgewebview2.exe" || name == "msedgewebview2") && parentHarness != nil {
		if ph, ok := parentHarness(p.PPID); ok && (ph == HarnessSelf || ph == HarnessSelfWebview) {
			return HarnessSelfWebview
		}
	}

	if parentHarness != nil {
		if h, ok := parentHarness(p.PPID); ok {
			return h
		}
	}

	if name == "node.exe" || name == "node" {
		return HarnessNodeUnclassified
	}
	return HarnessOther
}

func isVSCodeName(lowerName string) bool {
	return lowerName == "code.exe" || lowerName == "code - insiders.exe" || lowerName == "code"
}
