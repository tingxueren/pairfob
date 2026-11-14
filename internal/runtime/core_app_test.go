package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func appFixture() json.RawMessage {
	return json.RawMessage(`{"future":true,"spaces":[{"id":"sp1","name":"project","folder":"/repo","future":42,"tabs":[{"id":"tb1","name":"work","root":{"id":"tb1","type":"row","children":[{"id":"p1","type":"block","content":{"terminal":"t3"}},{"id":"p2","type":"block","content":{"terminal":"t4"}}]}}]}]}`)
}
func TestCoreAppSharedSplitProjection(t *testing.T) {
	fake := newFakeCoreTransport()
	fake.hello.Methods["doc.get"] = true
	fake.handler = func(method string, params json.RawMessage) (any, error) {
		switch method {
		case "terminal.list":
			return map[string]any{"terminals": []coreTerminal{{Address: "t3", State: "live", Cols: 80, Rows: 24, Cwd: "/repo"}, {Address: "t4", State: "live", Cols: 80, Rows: 24, Cwd: "/repo"}}}, nil
		case "doc.get":
			return map[string]any{"value": appFixture(), "version": 3}, nil
		}
		return coreFakeDefault(method, params)
	}
	c := newTestCore(t, fake)
	c.appDocs = map[string]coreAppDocument{"": {Value: appFixture()}}
	c.appDocAt = time.Now()
	v, err := c.Observe(context.Background(), DefaultSession(), SnapshotQuery{})
	if err != nil {
		t.Fatal(err)
	}
	s := v.(SnapshotView).Snapshot
	if len(s.Workspaces) != 1 || len(s.Tabs) != 1 || len(s.Panes) != 2 || len(s.Layouts) != 1 || len(s.Layouts[0].Panes) != 2 {
		t.Fatalf("lost App split: %+v", s)
	}
	if s.Panes[0].TabID != s.Panes[1].TabID || s.Tabs[0].Label != "work" || s.Workspaces[0].Label != "project" {
		t.Fatalf("identities %+v", s)
	}
	if s.Layouts[0].Panes[0].Rect.X >= s.Layouts[0].Panes[1].Rect.X {
		t.Fatal("row placement lost")
	}
}

func TestCoreAppUsesLiveWorkspaceNameAndCwd(t *testing.T) {
	doc := json.RawMessage(`{"spaces":[{"id":"sp1","name":"","folder":"/old/project","tabs":[{"id":"tb1","name":"work","root":{"id":"p1","type":"block","content":{"terminal":"t3"}}}]}]}`)
	terminals := []coreTerminal{{
		Address: "t3", State: "live", Cwd: "/old/project",
		Labels:     map[string]string{"herdr.space": "sp1", "where": "new-project / work"},
		Foreground: &coreForeground{Cwd: "/work/new-project"},
	}}
	snapshot := projectCoreApp(coreHello{}, terminals, map[string]coreAppDocument{"": {Value: doc}}, 1)
	if len(snapshot.Workspaces) != 1 || snapshot.Workspaces[0].Label != "new-project" || snapshot.Workspaces[0].Cwd != "/work/new-project" {
		t.Fatalf("workspace did not follow live App labels and process cwd: %+v", snapshot.Workspaces)
	}
	if len(snapshot.Panes) != 1 || snapshot.Panes[0].Cwd != "/work/new-project" {
		t.Fatalf("pane did not keep its live process cwd: %+v", snapshot.Panes)
	}
}

func TestCoreAppKeepsDocumentMetadataWhenLiveNamesDisagree(t *testing.T) {
	doc := json.RawMessage(`{"spaces":[{"id":"sp1","name":"document-name","folder":"/document/root","tabs":[{"id":"tb1","name":"work","root":{"id":"r","type":"row","children":[{"id":"p1","type":"block","content":{"terminal":"t3"}},{"id":"p2","type":"block","content":{"terminal":"t4"}}]}}]}]}`)
	terminals := []coreTerminal{
		{Address: "t3", State: "live", Labels: map[string]string{"where": "one / work"}},
		{Address: "t4", State: "live", Labels: map[string]string{"where": "two / work"}},
	}
	snapshot := projectCoreApp(coreHello{}, terminals, map[string]coreAppDocument{"": {Value: doc}}, 1)
	if len(snapshot.Workspaces) != 1 || snapshot.Workspaces[0].Label != "document-name" || snapshot.Workspaces[0].Cwd != "/document/root" {
		t.Fatalf("conflicting live names must preserve document metadata: %+v", snapshot.Workspaces)
	}
}

func TestCoreAppRemoteBlankMetadataUsesLiveWorkspaceName(t *testing.T) {
	doc := json.RawMessage(`{"spaces":[{"id":"sp317","name":"","folder":"","tabs":[{"id":"tb1","name":"小管维护","root":{"id":"p1","type":"block","content":{"terminal":"t4"}}}]}]}`)
	terminals := []coreTerminal{{
		Address: "mini/t4", State: "live", Cwd: "/Users/me",
		Labels:     map[string]string{"herdr.space": "sp317", "where": "lindorm-product / 小管维护"},
		Foreground: &coreForeground{Cwd: "/Users/me/Work/Git/lindorm-product"},
	}}
	snapshot := projectCoreApp(coreHello{}, terminals, map[string]coreAppDocument{"mini": {Value: doc}}, 1)
	if len(snapshot.Workspaces) != 1 || snapshot.Workspaces[0].Label != "mini · lindorm-product" {
		t.Fatalf("remote blank metadata did not use its live App label: %+v", snapshot.Workspaces)
	}
	if len(snapshot.Panes) != 1 || snapshot.Panes[0].Tokens["cwd"] != "/Users/me/Work/Git/lindorm-product" {
		t.Fatalf("remote pane did not preserve its display-only live cwd: %+v", snapshot.Panes)
	}
}

func TestCoreAppKeepsDocumentCwdWhenLiveCandidatesConflict(t *testing.T) {
	terminals := []coreTerminal{
		{Address: "t3", State: "live", Labels: map[string]string{"where": "repo / work"}, Foreground: &coreForeground{Cwd: "/checkout-a/repo"}},
		{Address: "t4", State: "live", Labels: map[string]string{"where": "repo / work"}, Foreground: &coreForeground{Cwd: "/checkout-b/repo"}},
	}
	for _, children := range []string{
		`[{"id":"p1","type":"block","content":{"terminal":"t3"}},{"id":"p2","type":"block","content":{"terminal":"t4"}}]`,
		`[{"id":"p2","type":"block","content":{"terminal":"t4"}},{"id":"p1","type":"block","content":{"terminal":"t3"}}]`,
	} {
		doc := json.RawMessage(`{"spaces":[{"id":"sp1","name":"","folder":"/old/project","tabs":[{"id":"tb1","name":"work","root":{"id":"r","type":"row","children":` + children + `}}]}]}`)
		snapshot := projectCoreApp(coreHello{}, terminals, map[string]coreAppDocument{"": {Value: doc}}, 1)
		if len(snapshot.Workspaces) != 1 || snapshot.Workspaces[0].Label != "repo" || snapshot.Workspaces[0].Cwd != "/old/project" {
			t.Fatalf("ambiguous live cwd must preserve document root regardless of layout order: %+v", snapshot.Workspaces)
		}
	}
}
