package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"pairfob/internal/runtime"
	"time"
)

func parseCoreAgentHook(args []string, input io.Reader) (runtime.AgentBindingReport, error) {
	var r runtime.AgentBindingReport
	if len(args) != 1 || (args[0] != "codex" && args[0] != "claude") {
		return r, errors.New("unsupported core hook")
	}
	data, err := io.ReadAll(io.LimitReader(input, 65537))
	if err != nil || len(data) > 65536 {
		return r, errors.New("oversized hook")
	}
	var e struct {
		SessionID string  `json:"session_id"`
		Event     string  `json:"hook_event_name"`
		AgentID   string  `json:"agent_id"`
		Parent    *string `json:"parent_agent_id"`
	}
	if json.Unmarshal(data, &e) != nil || e.SessionID == "" || e.AgentID != "" || e.Parent != nil {
		return r, errors.New("invalid root session hook")
	}
	if e.Event != "SessionStart" && e.Event != "SessionEnd" {
		return r, errors.New("unrelated hook")
	}
	if args[0] == "codex" {
		if inherited := os.Getenv("CODEX_THREAD_ID"); inherited != "" && inherited != e.SessionID {
			return r, errors.New("inherited thread mismatch")
		}
	}
	return runtime.AgentBindingReport{Agent: args[0], SessionID: e.SessionID, Sequence: time.Now().UnixNano(), Clear: e.Event == "SessionEnd"}, nil
}
