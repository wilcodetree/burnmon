package sysmon

import "testing"

func TestClassify(t *testing.T) {
	selfParent := func(pid int32) (Harness, bool) {
		if pid == 100 {
			return HarnessSelf, true
		}
		return "", false
	}
	otherParent := func(pid int32) (Harness, bool) {
		if pid == 200 {
			return HarnessCodex, true
		}
		return "", false
	}

	tests := []struct {
		name    string
		p       Proc
		otelSet bool
		parent  func(int32) (Harness, bool)
		want    Harness
	}{
		{
			name: "claude code cli by cmdline",
			p:    Proc{Name: "node.exe", Cmdline: `node C:\Users\x\AppData\Roaming\npm\node_modules\@anthropic-ai\claude-code\cli.js`},
			want: HarnessClaude,
		},
		{
			name: "claude code cli by process name",
			p:    Proc{Name: "claude.exe"},
			want: HarnessClaude,
		},
		{
			name: "claude desktop by cmdline",
			p:    Proc{Name: "Claude.exe", Cmdline: "Claude Desktop --type=renderer"},
			want: HarnessClaudeDesktop,
		},
		{
			name: "claude desktop by capitalized exe name heuristic",
			p:    Proc{Name: "Claude.exe"},
			want: HarnessClaudeDesktop,
		},
		{
			name: "codex by cmdline",
			p:    Proc{Name: "node.exe", Cmdline: `node C:\npm\node_modules\@openai\codex\bin\codex.js`},
			want: HarnessCodex,
		},
		{
			name: "codex by process name",
			p:    Proc{Name: "codex.exe"},
			want: HarnessCodex,
		},
		{
			name: "copilot cli by cmdline",
			p:    Proc{Name: "node.exe", Cmdline: "github-copilot-cli chat"},
			want: HarnessCopilotCLI,
		},
		{
			name: "copilot cli by process name",
			p:    Proc{Name: "copilot.exe"},
			want: HarnessCopilotCLI,
		},
		{
			name:    "copilot vscode when otel configured",
			p:       Proc{Name: "Code.exe", Cmdline: `extensionHost --copilot-chat`},
			otelSet: true,
			want:    HarnessCopilotVSCode,
		},
		{
			name:    "plain vs code stays unclassified without otel configured",
			p:       Proc{Name: "Code.exe", Cmdline: `extensionHost --copilot-chat`},
			otelSet: false,
			want:    HarnessOther,
		},
		{
			name: "hermes by name",
			p:    Proc{Name: "hermes"},
			want: HarnessHermes,
		},
		{
			name: "hermes by cmdline",
			p:    Proc{Name: "node.exe", Cmdline: "hermes-agent serve"},
			want: HarnessHermes,
		},
		{
			name: "wsl vmmem",
			p:    Proc{Name: "vmmem"},
			want: HarnessWSL,
		},
		{
			name: "wsl vmmemWSL",
			p:    Proc{Name: "vmmemWSL"},
			want: HarnessWSL,
		},
		{
			name: "burnmon-dev itself",
			p:    Proc{Name: "burnmon-dev.exe"},
			want: HarnessSelf,
		},
		{
			name:   "webview2 child of burnmon-dev inherits self",
			p:      Proc{Name: "msedgewebview2.exe", PPID: 100},
			parent: selfParent,
			want:   HarnessSelf,
		},
		{
			name:   "webview2 child of an unrelated app is other",
			p:      Proc{Name: "msedgewebview2.exe", PPID: 999},
			parent: selfParent,
			want:   HarnessOther,
		},
		{
			name:   "generic parent-chain inheritance",
			p:      Proc{Name: "conhost.exe", PPID: 200},
			parent: otherParent,
			want:   HarnessCodex,
		},
		{
			name: "unclassified node falls to its own bucket",
			p:    Proc{Name: "node.exe", Cmdline: "node server.js"},
			want: HarnessNodeUnclassified,
		},
		{
			name: "unrelated process is other",
			p:    Proc{Name: "explorer.exe"},
			want: HarnessOther,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.p, tc.otelSet, tc.parent)
			if got != tc.want {
				t.Errorf("Classify(%+v, otel=%v) = %q, want %q", tc.p, tc.otelSet, got, tc.want)
			}
		})
	}
}
