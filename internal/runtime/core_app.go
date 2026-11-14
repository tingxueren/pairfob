package runtime

// App documents own grouping and split trees. Terminal addresses remain the
// Pairfob pane identity; unknown document fields survive every write.
import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"
)

type coreAppDocument struct {
	Value   json.RawMessage `json:"value"`
	Version uint64          `json:"version"`
}
type coreAppNode struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Hidden  bool   `json:"hidden"`
	Content struct {
		Terminal string `json:"terminal"`
	} `json:"content"`
	Children []coreAppNode `json:"children"`
	Active   string        `json:"active"`
	Size     struct {
		Weight float64 `json:"weight"`
	} `json:"size"`
}
type coreAppSpace struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Folder string       `json:"folder"`
	Tabs   []coreAppTab `json:"tabs"`
}
type coreAppTab struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Root coreAppNode `json:"root"`
}
type coreAppModel struct {
	Spaces []coreAppSpace `json:"spaces"`
}

func coreAppTabID(machine, id string) string {
	if machine != "" {
		return "app:" + machine + ":" + id
	}
	return "app:" + id
}
func coreAppWorkspaceID(machine, id string) string {
	key := coreLocalWorkspaceID
	if machine != "" {
		key = coreMachinePrefix + machine
	}
	return key + ":space:" + id
}
func coreAppAddress(machine, id string) string {
	if machine != "" {
		return machine + "/" + id
	}
	return id
}
func coreAppMachines(ts []coreTerminal) []string {
	seen := map[string]bool{"": true}
	for _, t := range ts {
		seen[coreMachineOf(t.Address)] = true
	}
	out := []string{}
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
func coreReadApp(ctx context.Context, conn coreTransport, machine string) (coreAppDocument, error) {
	var d coreAppDocument
	params := map[string]any{"name": "herdr-app"}
	if machine != "" {
		params["on"] = machine
	}
	err := conn.Call(ctx, "doc.get", params, &d)
	return d, err
}

// Optional App metadata is refreshed away from the control connection. A slow
// machine must not stall typing or close an attach stream. The last valid
// documents remain useful while the live terminal intersection stays fresh.
func (c *Core) coreAppSnapshot(ctx context.Context, conn coreTransport, h coreHello, ts []coreTerminal, now int64) Snapshot {
	if !h.has("doc.get") {
		return projectCoreSnapshot(h, ts, now)
	}
	c.docMu.Lock()
	docs := map[string]coreAppDocument{}
	for m, d := range c.appDocs {
		docs[m] = d
	}
	if !c.appDocLoading && (c.appDocAt.IsZero() || time.Since(c.appDocAt) >= 2*time.Second) {
		c.appDocLoading = true
		machines := coreAppMachines(ts)
		go c.refreshCoreAppDocs(machines)
	}
	c.docMu.Unlock()
	return projectCoreApp(h, ts, docs, now)
}
func (c *Core) refreshCoreAppDocs(machines []string) {
	defer func() { c.docMu.Lock(); c.appDocLoading = false; c.appDocAt = time.Now(); c.docMu.Unlock() }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, h, err := c.independentConnection(ctx)
	if err != nil {
		return
	}
	defer conn.Close()
	if !h.has("doc.get") {
		return
	}
	docs := map[string]coreAppDocument{}
	for _, m := range machines {
		d, err := coreReadApp(ctx, conn, m)
		if err != nil {
			continue
		}
		docs[m] = d
	}
	c.docMu.Lock()
	if c.appDocs == nil {
		c.appDocs = map[string]coreAppDocument{}
	}
	for m, d := range docs {
		c.appDocs[m] = d
	}
	c.docMu.Unlock()
}
func projectCoreApp(hello coreHello, ts []coreTerminal, docs map[string]coreAppDocument, now int64) Snapshot {
	out := Snapshot{HerdrVersion: hello.Version, HerdrProtocol: int(hello.Generation), CapturedAt: now, Focused: map[string]string{}, Workspaces: []Workspace{}, Tabs: []Tab{}, Panes: []Pane{}}
	live := map[string]coreTerminal{}
	for _, t := range ts {
		live[t.Address] = t
	}
	used := map[string]bool{}
	for _, machine := range coreAppMachines(ts) {
		var model coreAppModel
		d := docs[machine]
		if len(d.Value) > 1<<20 || json.Unmarshal(d.Value, &model) != nil {
			continue
		}
		seenSpaces := map[string]bool{}
		seenTabs := map[string]bool{}
		for _, sp := range model.Spaces {
			if !coreAddressPartPattern.MatchString(sp.ID) || seenSpaces[sp.ID] {
				continue
			}
			seenSpaces[sp.ID] = true
			wid := coreAppWorkspaceID(machine, sp.ID)
			label := strings.TrimSpace(sp.Name)
			if label == "" {
				label = coreFolderName(sp.Folder)
			}
			if label == "" {
				label = sp.ID
			}
			if machine != "" {
				label = machine + " · " + label
			}
			w := Workspace{WorkspaceID: wid, Label: label}
			if machine == "" {
				w.Cwd = sp.Folder
			}
			statuses := []string{}
			for _, tab := range sp.Tabs {
				if !coreAddressPartPattern.MatchString(tab.ID) || seenTabs[tab.ID] {
					continue
				}
				seenTabs[tab.ID] = true
				nodes := []coreAppNode{}
				coreAppLeaves(tab.Root, &nodes, 0)
				members := []coreTerminal{}
				localUsed := map[string]bool{}
				for _, n := range nodes {
					a := coreAppAddress(machine, n.Content.Terminal)
					t, ok := live[a]
					if !ok || used[a] || localUsed[a] {
						continue
					}
					if !coreAddressPartPattern.MatchString(n.Content.Terminal) {
						continue
					}
					localUsed[a] = true
					members = append(members, t)
				}
				if len(members) == 0 {
					continue
				}
				tid := coreAppTabID(machine, tab.ID)
				layout := coreAppLayout(tab.Root, machine, members, wid, tid)
				out.Tabs = append(out.Tabs, Tab{TabID: tid, WorkspaceID: wid, Label: tab.Name})
				out.Layouts = append(out.Layouts, layout)
				for _, t := range members {
					used[t.Address] = true
					p := corePaneProjection(t, wid)
					p.TabID = tid
					out.Panes = append(out.Panes, p)
					statuses = append(statuses, p.AgentStatus)
				}
			}
			if len(statuses) > 0 {
				w.AgentStatus = coreWorkspaceAgentStatus(statuses)
				out.Workspaces = append(out.Workspaces, w)
			}
		}
	}
	rest := []coreTerminal{}
	for _, t := range ts {
		if !used[t.Address] {
			rest = append(rest, t)
		}
	}
	fallback := projectCoreSnapshot(hello, rest, now)
	workspaceIDs := map[string]int{}
	for i, w := range out.Workspaces {
		workspaceIDs[w.WorkspaceID] = i
	}
	for _, w := range fallback.Workspaces {
		if _, ok := workspaceIDs[w.WorkspaceID]; !ok {
			out.Workspaces = append(out.Workspaces, w)
		}
	}
	out.Tabs = append(out.Tabs, fallback.Tabs...)
	out.Panes = append(out.Panes, fallback.Panes...)
	out.Layouts = append(out.Layouts, fallback.Layouts...)
	for i := range out.Workspaces {
		statuses := []string{}
		for _, p := range out.Panes {
			if p.WorkspaceID == out.Workspaces[i].WorkspaceID {
				statuses = append(statuses, p.AgentStatus)
			}
		}
		out.Workspaces[i].AgentStatus = coreWorkspaceAgentStatus(statuses)
	}
	for i := range out.Workspaces {
		out.Workspaces[i].Number = i + 1
	}
	return out
}
func coreFolderName(folder string) string {
	s := strings.TrimRight(folder, "/\\")
	i := strings.LastIndexAny(s, "/\\")
	return s[i+1:]
}
func coreAppLeaves(n coreAppNode, out *[]coreAppNode, depth int) {
	if depth > 32 || len(*out) > 512 {
		return
	}
	if n.Type == "block" && n.Content.Terminal != "" {
		*out = append(*out, n)
	}
	for _, c := range n.Children {
		coreAppLeaves(c, out, depth+1)
	}
}
func coreAppLayout(root coreAppNode, machine string, members []coreTerminal, wid, tid string) TabLayout {
	allowed := map[string]bool{}
	cols, rows := 80, 24
	for _, t := range members {
		allowed[t.Address] = true
		if int(t.Cols) > cols {
			cols = int(t.Cols)
		}
		if int(t.Rows) > rows {
			rows = int(t.Rows)
		}
	}
	area := LayoutRect{Width: cols, Height: rows}
	out := TabLayout{WorkspaceID: wid, TabID: tid, Area: area, Panes: []LayoutPane{}}
	var walk func(coreAppNode, LayoutRect, int)
	walk = func(n coreAppNode, r LayoutRect, d int) {
		if d > 32 || n.Hidden {
			return
		}
		if n.Type == "block" {
			a := coreAppAddress(machine, n.Content.Terminal)
			if allowed[a] {
				out.Panes = append(out.Panes, LayoutPane{PaneID: encodeCoreAddress(a), Rect: r})
			}
			return
		}
		children := []coreAppNode{}
		for _, ch := range coreAppShownChildren(n) {
			ls := []coreAppNode{}
			coreAppShownLeaves(ch, &ls, 0)
			for _, leaf := range ls {
				if allowed[coreAppAddress(machine, leaf.Content.Terminal)] {
					children = append(children, ch)
					break
				}
			}
		}
		if len(children) == 0 {
			return
		}
		offset := 0
		total := r.Width
		if n.Type == "column" {
			total = r.Height
		}
		weights := make([]float64, len(children))
		sum := 0.0
		for i, ch := range children {
			weight := ch.Size.Weight
			if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
				weight = 1
			}
			weights[i] = weight
			sum += weight
		}
		for i, ch := range children {
			size := int(float64(total) * weights[i] / sum)
			if i == len(children)-1 {
				size = total - offset
			}
			if size < 0 {
				size = 0
			}
			box := r
			if n.Type == "column" {
				box.Y += offset
				box.Height = size
			} else if n.Type == "row" {
				box.X += offset
				box.Width = size
			}
			walk(ch, box, d+1)
			if n.Type != "stack" {
				offset += size
			}
		}
	}
	walk(root, area, 0)
	return out
}

// Membership includes hidden terminals; geometry includes only App-visible blocks.
func coreAppShownChildren(n coreAppNode) []coreAppNode {
	out := []coreAppNode{}
	for _, ch := range n.Children {
		if !ch.Hidden {
			out = append(out, ch)
		}
	}
	if n.Type == "stack" && len(out) > 0 {
		for _, ch := range out {
			if ch.ID == n.Active {
				return []coreAppNode{ch}
			}
		}
		return out[:1]
	}
	return out
}
func coreAppShownLeaves(n coreAppNode, out *[]coreAppNode, depth int) {
	if depth > 32 || n.Hidden || len(*out) > 512 {
		return
	}
	if n.Type == "block" && n.Content.Terminal != "" {
		*out = append(*out, n)
	}
	for _, ch := range coreAppShownChildren(n) {
		coreAppShownLeaves(ch, out, depth+1)
	}
}

func (c *Core) invalidateCoreAppDocs() { c.docMu.Lock(); c.appDocAt = time.Time{}; c.docMu.Unlock() }
