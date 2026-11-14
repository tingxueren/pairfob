package runtime

// Bindings are local hook observations, qualified by core identity and the
// live foreground process generation. Remote PIDs are never read locally.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func coreBindingPath(coreID, terminal string) (string, error) {
	if coreID == "" || !coreAddressPartPattern.MatchString(terminal) {
		return "", errors.New("invalid core binding identity")
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte("core\x00" + coreID + "\x00" + terminal))
	return filepath.Join(config, "pairfob-agent-bindings", hex.EncodeToString(sum[:])+".json"), nil
}
func coreBindingAgent(agent string) bool { return agent == "codex" || agent == "claude" }
func coreBindingPID(t coreTerminal, agent string) int {
	if t.State != "live" || t.Foreground == nil || t.Foreground.PID <= 1 || !nestedObservationAgent(t.Foreground.Argv, agent) {
		return 0
	}
	return t.Foreground.PID
}
func (c *Core) ReportAgentBinding(ctx context.Context, terminal string, report AgentBindingReport) error {
	if !coreAddressPartPattern.MatchString(terminal) || !coreBindingAgent(report.Agent) || !localSessionID.MatchString(report.SessionID) || report.Sequence <= 0 {
		return errors.New("invalid core binding report")
	}
	c.opMu.Lock()
	defer c.opMu.Unlock()
	conn, h, err := c.connection(ctx)
	if err != nil {
		return err
	}
	if !h.has("terminal.get") {
		return errors.New("core binding unavailable")
	}
	var t coreTerminal
	pid := 0
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err = conn.Call(ctx, "terminal.get", map[string]any{"terminal": terminal}, &t); err != nil {
			return c.endCall(conn, "binding", err, false, false)
		}
		if t.Address != terminal || t.State != "live" {
			return errors.New("core terminal changed")
		}
		pid = coreBindingPID(t, report.Agent)
		if pid > 0 && localHookDescendsFrom(ctx, pid, report.Agent) {
			break
		}
		if time.Now().After(deadline) {
			return errors.New("hook is outside terminal foreground agent")
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	birth, err := localProcessBirth(ctx, pid)
	if err != nil {
		return err
	}
	path, err := coreBindingPath(h.CoreID, terminal)
	if err != nil {
		return err
	}
	return saveLocalAgentBinding(path, localAgentBinding{Version: 1, Agent: report.Agent, SessionID: report.SessionID, TerminalID: terminal, ProcessID: pid, Birth: birth, Sequence: report.Sequence, Cleared: report.Clear})
}
func (c *Core) applyCoreAgentBindings(ctx context.Context, h coreHello, ts []coreTerminal, snapshot *Snapshot) {
	live := map[string]coreTerminal{}
	for _, t := range ts {
		live[t.Address] = t
	}
	for i := range snapshot.Panes {
		p := &snapshot.Panes[i]
		if coreMachineOf(p.TerminalID) != "" {
			continue
		}
		path, err := coreBindingPath(h.CoreID, p.TerminalID)
		if err != nil {
			continue
		}
		r, err := readLocalAgentBinding(path)
		if err != nil || r.Version != 1 || r.Cleared || !coreBindingAgent(r.Agent) || !localSessionID.MatchString(r.SessionID) || r.TerminalID != p.TerminalID {
			continue
		}
		t, ok := live[p.TerminalID]
		if !ok || coreBindingPID(t, r.Agent) != r.ProcessID {
			continue
		}
		birth, err := localProcessBirth(ctx, r.ProcessID)
		if err != nil || birth != r.Birth {
			continue
		}
		if p.Agent != "" && p.Agent != r.Agent {
			continue
		}
		p.Agent = r.Agent
		p.DisplayAgent = r.Agent
		p.AgentSession = &AgentSessionRef{Source: "pairfob:" + r.Agent, Agent: r.Agent, Kind: "id", Value: r.SessionID}
		p.AgentInstanceID = opaqueAgentInstanceID(h.CoreID+":"+p.TerminalID, p.AgentSession)
		p.HistoryAvailable = true
	}
}
