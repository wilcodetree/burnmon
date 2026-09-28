//go:build windows

package main

import "testing"

func TestTodoEnabledForRun_UicheckForcesOff(t *testing.T) {
	if got := todoEnabledForRun(true, true); got != false {
		t.Errorf("todoEnabledForRun(configEnabled=true, uicheckActive=true) = %v, want false", got)
	}
}

func TestTodoEnabledForRun_NoUicheckKeepsConfig(t *testing.T) {
	if got := todoEnabledForRun(true, false); got != true {
		t.Errorf("todoEnabledForRun(configEnabled=true, uicheckActive=false) = %v, want true", got)
	}
	if got := todoEnabledForRun(false, false); got != false {
		t.Errorf("todoEnabledForRun(configEnabled=false, uicheckActive=false) = %v, want false", got)
	}
}
