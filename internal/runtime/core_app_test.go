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
