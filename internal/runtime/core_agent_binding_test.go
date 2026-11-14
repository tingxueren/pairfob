package runtime

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestCoreBindingProcessGenerationAndCoreIdentity(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	pid := os.Getpid()
	birth, err := localProcessBirth(context.Background(), pid)
	if err != nil {
		t.Skip(err)
	}
	term := coreTerminal{Address: "t3", State: "live", Foreground: &coreForeground{PID: pid, Argv: []string{"codex"}}}
	path, err := coreBindingPath("core-a", "t3")
	if err != nil {
		t.Fatal(err)
	}
	r := localAgentBinding{Version: 1, Agent: "codex", SessionID: "session-123", TerminalID: "t3", ProcessID: pid, Birth: birth, Sequence: time.Now().UnixNano()}
	if saveLocalAgentBinding(path, r) != nil {
		t.Fatal("save")
	}
	for _, tc := range []struct {
		name, core, birth string
		clear             bool
		want              bool
	}{{"match", "core-a", birth, false, true}, {"differentcore", "core-b", birth, false, false}, {"restart", "core-a", "old-birth", false, false}, {"clear", "core-a", birth, true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			r.Birth = tc.birth
			r.Cleared = tc.clear
			r.Sequence++
			if err := saveLocalAgentBinding(path, r); err != nil {
				t.Fatal(err)
			}
			s := projectCoreSnapshot(coreHello{}, []coreTerminal{term}, 0)
			(&Core{}).applyCoreAgentBindings(context.Background(), coreHello{CoreID: tc.core}, []coreTerminal{term}, &s)
			if (s.Panes[0].AgentSession != nil) != tc.want {
				t.Fatal("wrong binding", s.Panes[0])
			}
			raw, _ := json.Marshal(s)
			if string(raw) == "" {
				t.Fatal("empty")
			}
		})
	}
}
func TestCoreHooksPreserveExistingConfiguration(t *testing.T) {
	root := t.TempDir()
	path := root + "/hooks.json"
	before := `{"unknown":9007199254740993,"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"user hook"}]}]}}`
	os.WriteFile(path, []byte(before), 0600)
	for i := 0; i < 2; i++ {
		if err := installCoreAgentHook(path, "/safe/pairfob", "codex"); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(path)
	var obj map[string]json.RawMessage
	json.Unmarshal(data, &obj)
	if string(obj["unknown"]) != "9007199254740993" {
		t.Fatal("unknown lost", string(data))
	}
	var h map[string][]any
	json.Unmarshal(obj["hooks"], &h)
	if len(h["SessionStart"]) != 2 || len(h["SessionEnd"]) != 1 {
		t.Fatal("duplicate/user hook lost", string(data))
	}
}
