package dataset

import (
	"reflect"
	"testing"
)

func TestDedupeCaseInsensitive(t *testing.T) {
	in := []string{
		`C:\Users\wilco\.claude\projects`,
		`c:\users\wilco\.claude\projects`,
		`C:\Users\wilco\AppData\Roaming\Claude\local-agent-mode-sessions`,
		`\\wsl.localhost\Ubuntu\home\wilco\.claude\projects`,
		`\\WSL.LOCALHOST\Ubuntu\home\wilco\.claude\projects`,
	}
	want := []string{
		`C:\Users\wilco\.claude\projects`,
		`C:\Users\wilco\AppData\Roaming\Claude\local-agent-mode-sessions`,
		`\\wsl.localhost\Ubuntu\home\wilco\.claude\projects`,
	}
	got := dedupeCaseInsensitive(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupeCaseInsensitive(%v) = %v, want %v", in, got, want)
	}
}
