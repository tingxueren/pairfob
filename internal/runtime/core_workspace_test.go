package runtime

import "testing"

func TestCoreProjectionAppWorkspaces(t *testing.T) {
	terminal := func(address, space, where, cwd string) coreTerminal {
		return coreTerminal{Address: address, State: "live", Cwd: cwd, Labels: map[string]string{"herdr.space": space, "where": where}}
	}
	items := []coreTerminal{
		terminal("t1", "sp1", "herdr / 1", "/repo/herdr"),
		terminal("t2", "sp1", "herdr / 2", "/other/folder"),
		terminal("t3", "sp2", "herdr / 1", "/repo/herdr"),
		terminal("nerd/t1", "sp1", "herdr / 1", "/remote/herdr"),
		terminal("t4", "", "", "/tmp"),
	}
	snapshot := projectCoreSnapshot(coreHello{Version: "0.1.21", Generation: 1}, items, 1)
	if len(snapshot.Workspaces) != 4 {
		t.Fatalf("want two local App projects, one local fallback and one remote project; got %+v", snapshot.Workspaces)
	}
	byPane := map[string]Pane{}
	for _, pane := range snapshot.Panes {
		byPane[pane.PaneID] = pane
	}
	first := byPane["t1"].WorkspaceID
	if first == "core" || byPane["t2"].WorkspaceID != first || byPane["t3"].WorkspaceID == first || byPane["nerd:t1"].WorkspaceID == first {
		t.Fatalf("project identities lost: %+v", byPane)
	}
	if byPane["t1"].TabID != "term:t1" || byPane["t2"].Cwd != "/other/folder" || byPane["nerd:t1"].Cwd != "" || byPane["nerd:t1"].Tokens["cwd"] != "/remote/herdr" {
		t.Fatalf("pane addressing or file roots changed: %+v", byPane)
	}
	labels := map[string]string{}
	for _, workspace := range snapshot.Workspaces {
		labels[workspace.WorkspaceID] = workspace.Label
	}
	if labels[first] != "herdr" || labels[byPane["nerd:t1"].WorkspaceID] != "nerd · herdr" || byPane["t4"].WorkspaceID != "core" {
		t.Fatalf("labels or fallback lost: %+v", labels)
	}
	items[0].Labels["where"] = "renamed project / 1"
	updated := projectCoreSnapshot(coreHello{}, items, 2)
	if updated.Workspaces[0].WorkspaceID != first || updated.Workspaces[0].Label != "renamed project" {
		t.Fatalf("rename changed identity or failed to refresh: %+v", updated.Workspaces)
	}
}

func TestCoreProjectionInvalidAppWorkspaceFallsBack(t *testing.T) {
	items := []coreTerminal{
		{Address: "t1", State: "live", Labels: map[string]string{"herdr.space": "bad/id", "where": "bad / 1"}},
		{Address: "t2", State: "live", Labels: map[string]string{"herdr.space": "sp2"}},
		{Address: "nerd/t1", State: "live", Labels: map[string]string{}},
	}
	snapshot := projectCoreSnapshot(coreHello{}, items, 1)
	if len(snapshot.Panes) != 3 || len(snapshot.Workspaces) != 3 || snapshot.Panes[0].WorkspaceID != "core" {
		t.Fatalf("unlabelled or invalid terminals dropped or misgrouped: %+v", snapshot)
	}
	if snapshot.Workspaces[1].Label != "sp2" || snapshot.Workspaces[2].Label != "nerd" {
		t.Fatalf("missing display metadata changed membership: %+v", snapshot.Workspaces)
	}
}

func TestCoreWorkspaceAmbiguousDisplayLabel(t *testing.T) {
	for _, where := range []string{"repo / API / auth", "group / repo / 1"} {
		key, label := coreTerminalWorkspace(coreTerminal{Address: "t1", Labels: map[string]string{"herdr.space": "sp1", "where": where}})
		if key != "core:space:sp1" || label != "sp1" {
			t.Fatalf("ambiguous label guessed: %s %s", key, label)
		}
	}
}
