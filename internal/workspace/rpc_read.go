package workspace

// ReadRPC preserves the public read shapes while reusing the same bounded
// inspector on the machine that owns the workspace. It grants no new root.
import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"unicode/utf8"
)

type ReadParams struct {
	Session *string `json:"session"`
	PaneID  string  `json:"pane_id"`
	Root    string  `json:"root"`
	Path    string  `json:"path"`
	Cursor  string  `json:"cursor"`
	Limit   int     `json:"limit"`
	Query   *string `json:"query"`
	Stats   *bool   `json:"stats"`
	Layer   string  `json:"layer"`
}

func ParseReadRPC(op string, raw json.RawMessage) (ReadParams, error) {
	var p ReadParams
	allowed := map[string]bool{"session": true, "pane_id": true}
	switch op {
	case "WorkspaceOpen", "GitBranches":
	case "GitStatus":
		allowed["stats"] = true
	case "WorkspaceList", "WorkspaceListAtRoot":
		for _, k := range []string{"path", "cursor", "limit"} {
			allowed[k] = true
		}
	case "WorkspaceRead", "WorkspaceReadAtRoot":
		allowed["path"] = true
	case "WorkspaceSearch":
		for _, k := range []string{"root", "query", "limit"} {
			allowed[k] = true
		}
	case "WorkspaceResolve":
		allowed["root"] = true
		allowed["path"] = true
	case "GitDiff":
		allowed["path"] = true
		allowed["layer"] = true
	default:
		return p, ErrNotFound
	}
	if op == "WorkspaceListAtRoot" || op == "WorkspaceReadAtRoot" {
		allowed["root"] = true
	}
	var fields map[string]json.RawMessage
	if len(raw) > 32768 || json.Unmarshal(raw, &fields) != nil {
		return p, ErrInvalidPath
	}
	for k := range fields {
		if !allowed[k] {
			return p, ErrInvalidPath
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if d.Decode(&p) != nil {
		return p, ErrInvalidPath
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return p, ErrInvalidPath
	}
	if p.PaneID == "" || p.Session != nil && *p.Session != "" {
		return p, ErrInvalidPath
	}
	if !utf8.ValidString(p.Path) || utf8.RuneCountInString(p.Path) > MaxPathRunes || len(p.Cursor) > 32 || p.Limit < 0 {
		return p, ErrInvalidPath
	}
	switch op {
	case "WorkspaceRead", "WorkspaceReadAtRoot", "WorkspaceResolve", "GitDiff":
		if p.Path == "" {
			return p, ErrInvalidPath
		}
	}
	if op == "WorkspaceList" || op == "WorkspaceListAtRoot" {
		if p.Limit > MaxListLimit {
			return p, ErrInvalidPath
		}
	}
	if op == "WorkspaceSearch" {
		if p.Query == nil || !utf8.ValidString(*p.Query) || utf8.RuneCountInString(*p.Query) > MaxSearchQueryRunes || p.Limit > MaxSearchLimit {
			return p, ErrInvalidPath
		}
	}
	if op == "GitDiff" && p.Layer != "worktree" && p.Layer != "staged" {
		return p, ErrInvalidPath
	}
	if op == "WorkspaceListAtRoot" || op == "WorkspaceReadAtRoot" || op == "WorkspaceSearch" || op == "WorkspaceResolve" {
		if p.Root == "" {
			return p, ErrInvalidPath
		}
	}
	return p, nil
}
func ReadRPC(op string, raw json.RawMessage, root string) (any, error) {
	p, err := ParseReadRPC(op, raw)
	if err != nil {
		return nil, err
	}
	if p.Root != "" {
		canonical, err := CanonicalRoot(root)
		if err != nil {
			return nil, err
		}
		if canonical != p.Root {
			return nil, ErrChanged
		}
	}
	i := NewInspector()
	switch op {
	case "WorkspaceOpen":
		return i.Describe(root)
	case "WorkspaceList", "WorkspaceListAtRoot":
		return i.List(root, p.Path, p.Cursor, p.Limit)
	case "WorkspaceRead", "WorkspaceReadAtRoot":
		return i.Read(root, p.Path)
	case "WorkspaceSearch":
		return i.Search(root, *p.Query, p.Limit)
	case "WorkspaceResolve":
		home, _ := os.UserHomeDir()
		return ResolveReference(root, p.Root, home, p.Path)
	case "GitStatus":
		if p.Stats != nil && *p.Stats {
			return i.StatusWithStats(root)
		}
		return i.Status(root)
	case "GitDiff":
		return i.Diff(root, p.Path, p.Layer)
	case "GitBranches":
		return i.Branches(root)
	}
	return nil, ErrNotFound
}
