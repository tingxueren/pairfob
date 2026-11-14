package runtime

import (
	"encoding/json"
	"testing"
)

func TestCoreAppWeightsAndHiddenStack(t *testing.T) {
	var root coreAppNode
	err := json.Unmarshal([]byte(`{"id":"root","type":"row","children":[{"id":"a","type":"block","size":{"weight":3},"content":{"terminal":"t1"}},{"id":"s","type":"stack","size":{"weight":1},"active":"b","children":[{"id":"hidden","type":"block","hidden":true,"content":{"terminal":"t2"}},{"id":"b","type":"block","content":{"terminal":"t3"}}]}]}`), &root)
	if err != nil {
		t.Fatal(err)
	}
	l := coreAppLayout(root, "", []coreTerminal{{Address: "t1", Cols: 80, Rows: 24}, {Address: "t2", Cols: 80, Rows: 24}, {Address: "t3", Cols: 80, Rows: 24}}, "core", "app:tb1")
	if len(l.Panes) != 2 || l.Panes[0].Rect.Width != 60 || l.Panes[1].PaneID != "t3" || l.Panes[1].Rect.X != 60 {
		t.Fatalf("layout %+v", l)
	}
}
func TestCoreAppOrphanDoesNotDuplicateWorkspace(t *testing.T) {
	ts := []coreTerminal{{Address: "t3", State: "live", Labels: map[string]string{"herdr.space": "sp1"}}, {Address: "t5", State: "live", Labels: map[string]string{"herdr.space": "sp1"}, Status: &coreStatus{State: "working", App: "codex"}}}
	s := projectCoreApp(coreHello{}, ts, map[string]coreAppDocument{"": {Value: appFixture()}}, 0)
	if len(s.Workspaces) != 1 || len(s.Panes) != 2 || s.Workspaces[0].AgentStatus != "working" {
		t.Fatalf("orphan lost %+v", s)
	}
}
