package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCoreLiveParityAppMutations(t *testing.T) {
	coreLiveGated(t, true)
	c := NewCore("")
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, h, err := c.connection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !h.has("doc.set") {
		t.Fatal("doc.set unavailable")
	}
	first := coreLiveCreate(t, ctx, c, "first")
	address, _ := decodeCorePaneID(first)
	doc := map[string]any{"future": json.Number("9007199254740993"), "spaces": []any{map[string]any{"id": "sp1", "name": "live", "folder": os.TempDir(), "tabs": []any{map[string]any{"id": "tb1", "name": "first", "root": map[string]any{"id": "p1", "type": "block", "content": map[string]any{"terminal": address}}}}}}}
	if err = coreAppSet(ctx, conn, "", coreAppDocument{}, doc); err != nil {
		t.Fatal(err)
	}
	c.refreshCoreAppDocs([]string{""})
	ratio := .7
	if _, err = c.Execute(ctx, DefaultSession(), liveOperation("split"), SplitPaneCommand{WorkspaceID: "core:space:sp1", TargetTabID: "app:tb1", TargetPaneID: first, Direction: SplitRight, Ratio: &ratio}); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Execute(ctx, DefaultSession(), liveOperation("rename"), RenameTabCommand{TabID: "app:tb1", Label: "phone"}); err != nil {
		t.Fatal(err)
	}
	c.refreshCoreAppDocs([]string{""})
	v, err := c.Observe(ctx, DefaultSession(), SnapshotQuery{})
	if err != nil {
		t.Fatal(err)
	}
	s := v.(SnapshotView).Snapshot
	if len(s.Workspaces) != 1 || len(s.Tabs) != 1 || len(s.Panes) != 2 || s.Tabs[0].Label != "phone" || len(s.Layouts[0].Panes) != 2 {
		t.Fatal("live App mismatch", s)
	}
	d, err := coreReadApp(ctx, conn, "")
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	json.Unmarshal(d.Value, &raw)
	if string(raw["future"]) != "9007199254740993" {
		t.Fatal("unknown changed", string(d.Value))
	}
	t.Log("real PTY split + rename + exact unknown fields passed")
}
func TestCoreLiveParityWorkspaceHelper(t *testing.T) {
	coreLiveGated(t, true)
	binary := os.Getenv("PAIRFOB_PARITY_TEST_BINARY")
	if binary == "" {
		t.Skip("candidate binary required")
	}
	c := NewCore("")
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "note.txt"), []byte("owned remote-style helper"), 0600)
	conn, _, err := c.connection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var run coreRunResult
	if err = conn.Call(ctx, "terminal.run", map[string]any{"argv": []string{"sh", "-c", "sleep 30"}, "cwd": root, "no_parent": true}, &run); err != nil {
		t.Fatal(err)
	}
	var terminal coreTerminal
	conn.Call(ctx, "terminal.get", map[string]any{"terminal": run.Address}, &terminal)
	q := WorkspaceReadHelperRequest{Version: 1, Terminal: run.Address, Cwd: root, PID: terminal.PID, Started: terminal.ProgramStartedMS, Operation: "WorkspaceRead", Params: json.RawMessage(`{"pane_id":"dev:` + run.Address + `","path":"note.txt"}`)}
	data, _ := json.Marshal(q)
	process := exec.CommandContext(ctx, binary, "workspace-read-helper")
	process.Stdin = bytes.NewReader(data)
	out, err := process.Output()
	if err != nil {
		t.Fatal(err)
	}
	var reply workspaceReadHelperResponse
	if json.Unmarshal(out, &reply) != nil || reply.Error != "" {
		t.Fatal(string(out))
	}
	q.PID++
	data, _ = json.Marshal(q)
	process = exec.CommandContext(ctx, binary, "workspace-read-helper")
	process.Stdin = bytes.NewReader(data)
	out, err = process.Output()
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(out, &reply)
	if reply.Error != "changed" {
		t.Fatal("wrong PID accepted", string(out))
	}
	t.Log("real helper read + stale terminal rejection passed")
}

func TestCoreLiveParitySessionHook(t *testing.T) {
	coreLiveGated(t, true)
	binary := os.Getenv("PAIRFOB_PARITY_TEST_BINARY")
	if binary == "" {
		t.Skip("candidate required")
	}
	root := t.TempDir()
	script := filepath.Join(root, "codex")
	body := "#!/bin/sh\nprintf " + shellQuoteHook(`{"hook_event_name":"SessionStart","session_id":"session-live-123"}`) + " | " + shellQuoteHook(binary) + " agent-hook codex\nsleep 20\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	c := NewCore("")
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := c.connection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var run coreRunResult
	if err = conn.Call(ctx, "terminal.run", map[string]any{"argv": []string{script}, "cwd": root, "no_parent": true}, &run); err != nil {
		t.Fatal(err)
	}
	for {
		v, err := c.Observe(ctx, DefaultSession(), SnapshotQuery{})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range v.(SnapshotView).Snapshot.Panes {
			if p.TerminalID == run.Address && p.AgentSession != nil && p.AgentSession.Value == "session-live-123" {
				t.Log("real foreground process + SessionStart hook binding passed; no paid agent started")
				return
			}
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			t.Fatal("real hook did not bind")
		case <-timer.C:
		}
	}
}

func TestCoreLiveParityRemoteMachineWorkspace(t *testing.T) {
	coreLiveGated(t, true)
	binary := os.Getenv("PAIRFOB_PARITY_TEST_BINARY")
	if binary == "" {
		t.Skip("candidate required")
	}
	scratch, err := os.MkdirTemp("/var/tmp", "pfr.")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(scratch)
	home := scratch + "/home"
	os.MkdirAll(home+"/.local/bin", 0700)
	hc, _ := filepath.Abs(os.Getenv("PAIRFOB_HC"))
	os.Symlink(hc, home+"/.local/bin/herdr-core")
	os.Symlink(hc, home+"/.local/bin/hc")
	os.Symlink(binary, home+"/.local/bin/pairfob")
	env := []string{"HOME=" + home, "PATH=" + home + "/.local/bin:/usr/bin:/bin", "HC_SESSION=pfr", "HC_SOCKET=" + scratch + "/core.sock", "XDG_STATE_HOME=" + scratch + "/state", "XDG_CONFIG_HOME=" + scratch + "/config", "XDG_RUNTIME_DIR=" + scratch + "/run"}
	remote := exec.Command(hc, "server", "start")
	remote.Env = append(os.Environ(), env...)
	if out, err := remote.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	defer func() { p := exec.Command(hc, "server", "stop"); p.Env = append(os.Environ(), env...); p.Run() }()
	via := scratch + "/via"
	body := "#!/bin/sh\nexec env"
	for _, v := range env {
		body += " " + shellQuoteHook(v)
	}
	body += " sh -c \"$1\"\n"
	os.WriteFile(via, []byte(body), 0700)
	c := NewCore("")
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	conn, _, err := c.connection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var added map[string]any
	if err = conn.Call(ctx, "machine.add", map[string]any{"name": "parityremote", "via": via, "session": "pfr", "temporary": true}, &added); err != nil {
		t.Fatal(err)
	}
	root := home + "/repo"
	os.Mkdir(root, 0700)
	os.WriteFile(root+"/note.txt", []byte("remote-machine-owned"), 0600)
	var run coreRunResult
	for {
		err = conn.Call(ctx, "terminal.run", map[string]any{"on": "parityremote", "cwd": root, "argv": []string{"sh", "-c", "sleep 30"}, "no_parent": true}, &run)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(err)
		case <-time.After(100 * time.Millisecond):
		}
	}
	raw, _ := json.Marshal(map[string]any{"pane_id": encodeCoreAddress(run.Address), "path": "note.txt"})
	result, handled, err := c.ReadRemoteWorkspace(ctx, DefaultSession(), "WorkspaceRead", raw)
	if err != nil || !handled || !bytes.Contains(result, []byte("remote-machine-owned")) {
		t.Fatal("machine route", string(result), handled, err)
	}
	t.Log("independent target core + machine.exec + target fork helper end-to-end passed")
}
