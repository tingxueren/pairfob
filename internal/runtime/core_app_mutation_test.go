package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func appMutationFake() *fakeCoreTransport {
	f := newFakeCoreTransport()
	f.hello.Methods["doc.get"] = true
	f.hello.Methods["doc.set"] = true
	f.handler = func(method string, p json.RawMessage) (any, error) {
		switch method {
		case "doc.get":
			return coreAppDocument{Value: appFixture(), Version: 3}, nil
		case "doc.set":
			return map[string]any{"version": 4}, nil
		case "terminal.run":
			return coreRunResult{Address: "t5"}, nil
		}
		return coreFakeDefault(method, p)
	}
	return f
}
func TestCoreAppCrossWorkspaceNoMutation(t *testing.T) {
	f := appMutationFake()
	base := f.handler
	f.handler = func(m string, p json.RawMessage) (any, error) {
		if m == "doc.get" {
			raw := strings.Replace(string(appFixture()), `"tabs":[`, `"tabs":[]},{"id":"sp2","tabs":[`, 1)
			return coreAppDocument{Value: json.RawMessage(raw), Version: 3}, nil
		}
		return base(m, p)
	}
	_, err := newTestCore(t, f).Execute(context.Background(), DefaultSession(), "cross", SplitPaneCommand{WorkspaceID: "core:space:sp1", TargetTabID: "app:tb1", TargetPaneID: "t3", Direction: SplitRight})
	if err == nil || len(f.callsOf("terminal.run")) != 0 || len(f.callsOf("doc.set")) != 0 {
		t.Fatalf("cross-space write: %v", err)
	}
}
func TestCoreAppCASDoesNotReplayCreatedTerminal(t *testing.T) {
	f := appMutationFake()
	base := f.handler
	f.handler = func(m string, p json.RawMessage) (any, error) {
		if m == "doc.set" {
			return nil, &coreRPCError{Kind: "conflict", Message: "version mismatch"}
		}
		return base(m, p)
	}
	r, err := newTestCore(t, f).Execute(context.Background(), DefaultSession(), "cas", CreateTabCommand{WorkspaceID: "core:space:sp1", Label: "new"})
	fault, ok := AsFault(err)
	if !ok || r.Outcome != OutcomeUnknown || fault.Retry != RetryNever || len(f.callsOf("terminal.run")) != 1 || len(f.callsOf("doc.set")) != 1 {
		t.Fatalf("replay outcome %+v %v", r, err)
	}
}
func TestCoreAppRenamePreservesUnknownInteger(t *testing.T) {
	f := appMutationFake()
	base := f.handler
	f.handler = func(m string, p json.RawMessage) (any, error) {
		if m == "doc.get" {
			raw := strings.Replace(string(appFixture()), `"future":true`, `"future":9007199254740993`, 1)
			return coreAppDocument{Value: json.RawMessage(raw), Version: 3}, nil
		}
		return base(m, p)
	}
	r, err := newTestCore(t, f).Execute(context.Background(), DefaultSession(), "rename", RenameTabCommand{TabID: "app:tb1", Label: "next"})
	if err != nil || r.Outcome != OutcomeApplied {
		t.Fatal(r, err)
	}
	calls := f.callsOf("doc.set")
	if len(calls) != 1 || !strings.Contains(string(calls[0]), `9007199254740993`) {
		t.Fatalf("lost unknown integer %s", calls)
	}
}
func TestCoreAppSplitKeepsSlotAndRatio(t *testing.T) {
	root := map[string]any{"id": "p1", "type": "block", "size": map[string]any{"weight": 3}, "content": map[string]any{"terminal": "t3"}, "future": true}
	next := map[string]any{"id": "p2", "type": "block", "content": map[string]any{"terminal": "t4"}}
	if !coreAppSplitNode(root, "t3", next, "column", "split1", .7, 0) {
		t.Fatal("split failed")
	}
	size, _ := appObject(root["size"])
	if size["weight"] != 3 {
		t.Fatal("slot lost", root)
	}
	old, _ := appObject(appArray(root["children"])[0])
	weights, _ := appObject(old["size"])
	if weights["weight"] != .7 || old["future"] != true {
		t.Fatal("child lost", root)
	}
}
func TestCoreAppMutationMissingMethods(t *testing.T) {
	f := appMutationFake()
	delete(f.hello.Methods, "doc.set")
	_, err := newTestCore(t, f).Execute(context.Background(), DefaultSession(), "missing", CreateTabCommand{WorkspaceID: "core:space:sp1"})
	var fault *Fault
	if !errors.As(err, &fault) || fault.Code != CodeUnsupported || len(f.callLog()) != 0 {
		t.Fatal("missing method wrote", err)
	}
}
