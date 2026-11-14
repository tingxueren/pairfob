package main

import (
	"strings"
	"testing"
)

func TestCoreHookOnlyRootSessionEvents(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "")
	for _, event := range []string{"SessionStart", "SessionEnd"} {
		r, err := parseCoreAgentHook([]string{"codex"}, strings.NewReader(`{"hook_event_name":"`+event+`","session_id":"12345678"}`))
		if err != nil || r.Clear != (event == "SessionEnd") {
			t.Fatal(r, err)
		}
	}
	for _, raw := range []string{`{"hook_event_name":"UserPromptSubmit","session_id":"12345678"}`, `{"hook_event_name":"SessionStart","session_id":"12345678","parent_agent_id":"nested"}`} {
		if _, err := parseCoreAgentHook([]string{"claude"}, strings.NewReader(raw)); err == nil {
			t.Fatal("accepted nested/unrelated")
		}
	}
	t.Setenv("CODEX_THREAD_ID", "other")
	if _, err := parseCoreAgentHook([]string{"codex"}, strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"12345678"}`)); err == nil {
		t.Fatal("inherited mismatch")
	}
}
