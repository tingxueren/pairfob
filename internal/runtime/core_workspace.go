package runtime

import "strings"

// App identity labels, not cwd or display text, decide membership. Names and
// folders can change; terminal addresses and Pairfob one-pane tab IDs stay put.
// No App document is written and no terminal is moved by this projection.
type coreWorkspaceGroup struct {
	key       string
	label     string
	machine   string
	terminals []coreTerminal
}

func coreWhereWorkspaceLabel(terminal coreTerminal) string {
	label := strings.TrimSpace(terminal.Labels["where"])
	if strings.Count(label, " / ") > 1 {
		// The text is not escaped; either name can contain the separator.
		// Never guess a project name from an ambiguous display label.
		return ""
	}
	if cut := strings.Index(label, " / "); cut >= 0 {
		label = strings.TrimSpace(label[:cut])
	}
	return label
}

func coreTerminalWorkspace(terminal coreTerminal) (key, label string) {
	key = coreWorkspaceKey(terminal.Address)
	label = coreWorkspaceLabel(key)
	space := terminal.Labels["herdr.space"]
	if !coreAddressPartPattern.MatchString(space) {
		return key, label
	}
	key += ":space:" + space
	// App writes where as <space name> / <tab name>. Keep it display-only:
	// missing text still gets its own group, and duplicate names never merge.
	label = coreWhereWorkspaceLabel(terminal)
	if label == "" {
		label = space
	}
	if machine := coreMachineOf(terminal.Address); machine != "" {
		label = machine + " · " + label
	}
	return key, label
}

func coreWorkspaceGroups(terminals []coreTerminal) []coreWorkspaceGroup {
	groups := make([]coreWorkspaceGroup, 0)
	indices := make(map[string]int)
	for _, terminal := range terminals {
		if terminal.State != "live" {
			continue
		}
		key, label := coreTerminalWorkspace(terminal)
		index, exists := indices[key]
		if !exists {
			index = len(groups)
			indices[key] = index
			groups = append(groups, coreWorkspaceGroup{key: key, label: label, machine: coreMachineOf(terminal.Address)})
		}
		groups[index].terminals = append(groups[index].terminals, terminal)
	}
	// Local projects first, then remote projects, preserving the terminal
	// list ordering within each half and the original legacy machine order.
	out := make([]coreWorkspaceGroup, 0, len(groups))
	for _, group := range groups {
		if group.machine == "" {
			out = append(out, group)
		}
	}
	for _, group := range groups {
		if group.machine != "" {
			out = append(out, group)
		}
	}
	return out
}
