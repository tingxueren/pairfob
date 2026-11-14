package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoreOptionalDocFailureKeepsControl(t *testing.T) {
	control := newFakeCoreTransport()
	control.hello.Methods["doc.get"] = true
	metadata := newFakeCoreTransport()
	metadata.hello.Methods["doc.get"] = true
	finished := make(chan struct{})
	metadata.handler = func(m string, p json.RawMessage) (any, error) {
		if m == "doc.get" {
			close(finished)
			metadata.Close()
			return nil, context.DeadlineExceeded
		}
		return coreFakeDefault(m, p)
	}
	c := newTestCore(t, control)
	var dials atomic.Int32
	c.dial = func(context.Context) (coreTransport, coreHello, error) {
		if dials.Add(1) == 1 {
			return control, control.hello, nil
		}
		return metadata, metadata.hello, nil
	}
	if _, err := c.Observe(context.Background(), DefaultSession(), SnapshotQuery{}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("refresh missing")
	}
	if _, err := c.Execute(context.Background(), DefaultSession(), "input", SendTextCommand{PaneID: "t3", Text: "test"}); err != nil || control.closeCount() != 0 {
		t.Fatal("optional read broke input", err)
	}
}
func TestCoreSlowRemoteHelperDoesNotBlockInput(t *testing.T) {
	control := newFakeCoreTransport()
	remote := newFakeCoreTransport()
	remote.hello.Methods["machine.exec"] = true
	entered := make(chan struct{})
	release := make(chan struct{})
	remote.handler = func(m string, p json.RawMessage) (any, error) {
		switch m {
		case "terminal.get":
			return coreTerminal{Address: "dev/t3", State: "live", Cwd: "/remote", PID: 100}, nil
		case "machine.exec":
			close(entered)
			<-release
			return nil, errors.New("offline")
		}
		return coreFakeDefault(m, p)
	}
	c := newTestCore(t, control)
	c.conn = control
	c.hello = control.hello
	c.dial = func(context.Context) (coreTransport, coreHello, error) { return remote, remote.hello, nil }
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.ReadRemoteWorkspace(context.Background(), DefaultSession(), "WorkspaceOpen", json.RawMessage(`{"pane_id":"dev:t3"}`))
	}()
	<-entered
	input := make(chan error, 1)
	go func() {
		_, err := c.Execute(context.Background(), DefaultSession(), "input", SendTextCommand{PaneID: "t3", Text: "test"})
		input <- err
	}()
	select {
	case err := <-input:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("remote query blocked input")
	}
	close(release)
	<-done
}
func TestCoreHookWaitsForForegroundRefresh(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	f := newFakeCoreTransport()
	calls := 0
	f.handler = func(m string, p json.RawMessage) (any, error) {
		if m == "terminal.get" {
			calls++
			argv := []string{"sh"}
			if calls > 1 {
				argv = []string{"codex"}
			}
			return coreTerminal{Address: "t3", State: "live", Foreground: &coreForeground{PID: os.Getpid(), Argv: argv}}, nil
		}
		return coreFakeDefault(m, p)
	}
	err := newTestCore(t, f).ReportAgentBinding(context.Background(), "t3", AgentBindingReport{Agent: "codex", SessionID: "session-123", Sequence: time.Now().UnixNano()})
	if err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatal("foreground was not rechecked")
	}
}
