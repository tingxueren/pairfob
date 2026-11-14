package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// App identity labels, not cwd or display text, decide membership. Names and
// folders can change; terminal addresses and Pairfob one-pane tab IDs stay put.
// No App document is written and no terminal is moved by this projection.
type coreWorkspaceGroup struct {
	key       string
	label     string
	machine   string
	cwd       string
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

func coreProjectCwd(terminal coreTerminal) string {
	raw := coreResolvedCwd(terminal)
	if strings.TrimSpace(raw) == "" || !filepath.IsAbs(raw) {
		return ""
	}
	cwd := filepath.Clean(raw)
	if cwd == "." || cwd == string(filepath.Separator) {
		return ""
	}
	slash := filepath.ToSlash(cwd)
	parts := strings.Split(strings.Trim(slash, "/"), "/")
	// Only promote a directory below a Unix user home. A bare home, /tmp,
	// /repo and other generic roots remain in the machine fallback group.
	if len(parts) < 3 || (parts[0] != "Users" && parts[0] != "home") {
		return ""
	}
	return cwd
}

func coreCwdWorkspaceKey(address, cwd string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(cwd)))
	key := coreWorkspaceKey(address)
	return key + ":cwd:" + hex.EncodeToString(sum[:8])
}

func coreTerminalWorkspace(terminal coreTerminal) (key, label, cwd string) {
	key = coreWorkspaceKey(terminal.Address)
	label = coreWorkspaceLabel(key)
	space := terminal.Labels["herdr.space"]
	if !coreAddressPartPattern.MatchString(space) {
		if cwd = coreProjectCwd(terminal); cwd != "" {
			key = coreCwdWorkspaceKey(terminal.Address, cwd)
			label = filepath.Base(cwd)
			if machine := coreMachineOf(terminal.Address); machine != "" {
				label = machine + " · " + label
			}
		}
		return key, label, cwd
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
	return key, label, ""
}

func coreWorkspaceGroups(terminals []coreTerminal) []coreWorkspaceGroup {
	groups := make([]coreWorkspaceGroup, 0)
	indices := make(map[string]int)
	for _, terminal := range terminals {
		if terminal.State != "live" {
			continue
		}
		key, label, cwd := coreTerminalWorkspace(terminal)
		index, exists := indices[key]
		if !exists {
			index = len(groups)
			indices[key] = index
			groups = append(groups, coreWorkspaceGroup{key: key, label: label, machine: coreMachineOf(terminal.Address), cwd: cwd})
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
