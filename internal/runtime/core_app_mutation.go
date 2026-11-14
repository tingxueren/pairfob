package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
)

func coreAppIdentity(id string, tab bool) (machine, local string, ok bool) {
	prefix := "core:space:"
	if tab {
		prefix = "app:"
	}
	if strings.HasPrefix(id, prefix) {
		parts := strings.Split(strings.TrimPrefix(id, prefix), ":")
		if tab && len(parts) == 2 {
			return parts[0], parts[1], coreAddressPartPattern.MatchString(parts[0]) && coreAddressPartPattern.MatchString(parts[1])
		}
		if len(parts) == 1 {
			return "", parts[0], coreAddressPartPattern.MatchString(parts[0])
		}
	}
	if !tab && strings.HasPrefix(id, "machine:") {
		parts := strings.Split(id, ":")
		if len(parts) == 4 && parts[2] == "space" {
			return parts[1], parts[3], coreAddressPartPattern.MatchString(parts[1]) && coreAddressPartPattern.MatchString(parts[3])
		}
	}
	return "", "", false
}
func coreFreshID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b[:]), nil
}
func appObject(v any) (map[string]any, bool) { m, ok := v.(map[string]any); return m, ok }
func appArray(v any) []any                   { a, _ := v.([]any); return a }
func appText(v any) string                   { s, _ := v.(string); return s }
func coreAppLoad(ctx context.Context, conn coreTransport, machine string) (coreAppDocument, map[string]any, error) {
	d, err := coreReadApp(ctx, conn, machine)
	if err != nil {
		return d, nil, err
	}
	var obj map[string]any
	if string(d.Value) == "null" || len(d.Value) == 0 {
		obj = map[string]any{"spaces": []any{}}
	} else {
		if len(d.Value) > 1<<20 {
			return d, nil, errors.New("oversized App document")
		}
		dec := json.NewDecoder(bytes.NewReader(d.Value))
		dec.UseNumber()
		if err = dec.Decode(&obj); err != nil || obj == nil {
			return d, nil, errors.New("invalid App document")
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			return d, nil, errors.New("trailing App document data")
		}
	}
	return d, obj, nil
}
func coreAppFind(obj map[string]any, id string, tab bool) (map[string]any, map[string]any) {
	for _, v := range appArray(obj["spaces"]) {
		s, ok := appObject(v)
		if !ok {
			continue
		}
		if !tab && appText(s["id"]) == id {
			return s, nil
		}
		for _, v := range appArray(s["tabs"]) {
			t, ok := appObject(v)
			if ok && appText(t["id"]) == id && tab {
				return s, t
			}
		}
	}
	return nil, nil
}
func coreAppSet(ctx context.Context, conn coreTransport, machine string, d coreAppDocument, obj map[string]any) error {
	p := map[string]any{"name": "herdr-app", "version": d.Version, "value": obj}
	if machine != "" {
		p["on"] = machine
	}
	var result struct {
		Version uint64 `json:"version"`
	}
	if err := conn.Call(ctx, "doc.set", p, &result); err != nil {
		return err
	}
	if result.Version != d.Version+1 {
		return coreProtocol("doc.set", "invalid document version")
	}
	return nil
}
func (c *Core) coreAppRename(ctx context.Context, conn coreTransport, h coreHello, op, id, label string, tab bool) (Receipt, error) {
	m, local, ok := coreAppIdentity(id, tab)
	if !ok {
		return notApplied(op, unsupported("rename", "not an App-managed object"))
	}
	if !h.has("doc.get") || !h.has("doc.set") {
		return notApplied(op, unsupported("rename", "App document editing unsupported"))
	}
	d, obj, err := coreAppLoad(ctx, conn, m)
	if err != nil {
		return coreFailure(op, c.endCall(conn, "rename", err, false, false))
	}
	s, t := coreAppFind(obj, local, tab)
	target := s
	kind := EntityWorkspace
	if tab {
		target = t
		kind = EntityTab
	}
	if target == nil {
		return notApplied(op, invalidFault("rename", "App object no longer exists"))
	}
	target["name"] = label
	if err = coreAppSet(ctx, conn, m, d, obj); err != nil {
		return coreFailure(op, c.endCall(conn, "rename", err, true, true))
	}
	c.invalidateCoreAppDocs()
	return Receipt{OperationID: op, Outcome: OutcomeApplied, Updated: []EntityRef{{Kind: kind, ID: id}}}, nil
}
func coreAppSplitNode(n map[string]any, terminal string, newNode map[string]any, direction, id string, ratio float64, depth int) bool {
	if depth > 32 {
		return false
	}
	if content, ok := appObject(n["content"]); ok && appText(content["terminal"]) == terminal {
		old := map[string]any{}
		for k, v := range n {
			old[k] = v
		}
		for k := range n {
			delete(n, k)
		}
		n["size"] = old["size"]
		delete(old, "size")
		old["size"] = map[string]any{"weight": ratio}
		newNode["size"] = map[string]any{"weight": 1 - ratio}
		n["id"] = id
		n["type"] = direction
		n["children"] = []any{old, newNode}
		return true
	}
	for _, v := range appArray(n["children"]) {
		if ch, ok := appObject(v); ok && coreAppSplitNode(ch, terminal, newNode, direction, id, ratio, depth+1) {
			return true
		}
	}
	return false
}
func (c *Core) coreAppCreate(ctx context.Context, conn coreTransport, h coreHello, op string, cmd CreateTabCommand, split *SplitPaneCommand) (Receipt, error) {
	m, space, ok := coreAppIdentity(cmd.WorkspaceID, false)
	if !ok {
		return notApplied(op, unsupported("create", "select an App-managed workspace"))
	}
	for _, method := range []string{"doc.get", "doc.set", "terminal.run"} {
		if !h.has(method) {
			return notApplied(op, unsupported("create", "App document editing unsupported"))
		}
	}
	d, obj, err := coreAppLoad(ctx, conn, m)
	if err != nil {
		return coreFailure(op, c.endCall(conn, "create", err, false, false))
	}
	s, _ := coreAppFind(obj, space, false)
	if s == nil {
		return notApplied(op, invalidFault("create", "workspace no longer exists"))
	}
	var target map[string]any
	localTerminal := ""
	tid := ""
	direction := "row"
	ratio := 0.5
	splitID, err := coreFreshID("split")
	if err != nil {
		return notApplied(op, err)
	}
	if split != nil {
		sm, lt, valid := coreAppIdentity(split.TargetTabID, true)
		if !valid || sm != m {
			return notApplied(op, invalidFault("split", "invalid App tab"))
		}
		owner, foundTab := coreAppFind(obj, lt, true)
		target = foundTab
		if owner == nil || appText(owner["id"]) != space {
			return notApplied(op, invalidFault("split", "tab belongs to another workspace"))
		}
		if split.Ratio != nil {
			ratio = *split.Ratio
			if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0.1 || ratio > 0.9 {
				return notApplied(op, invalidFault("split", "ratio must be between 0.1 and 0.9"))
			}
		}
		address, e := decodeCorePaneID(split.TargetPaneID)
		if e != nil || coreMachineOf(address) != m || target == nil {
			return notApplied(op, invalidFault("split", "target is not in workspace"))
		}
		localTerminal = strings.TrimPrefix(address, m+"/")
		if m == "" {
			localTerminal = address
		}
		tid = lt
		var root coreAppNode
		b, _ := json.Marshal(target["root"])
		_ = json.Unmarshal(b, &root)
		ls := []coreAppNode{}
		coreAppLeaves(root, &ls, 0)
		found := false
		for _, n := range ls {
			if n.Content.Terminal == localTerminal {
				found = true
			}
		}
		if !found {
			return notApplied(op, invalidFault("split", "target pane not in tab"))
		}
		if split.Direction == SplitDown {
			direction = "column"
		} else if split.Direction != SplitRight {
			return notApplied(op, invalidFault("split", "invalid direction"))
		}
	}
	if cmd.CWD == "" {
		cmd.CWD = appText(s["folder"])
	}
	params, err := coreConversationParams(CreateConversationCommand{CWD: cmd.CWD, AgentKind: cmd.AgentKind, Label: cmd.Label}, "")
	if err != nil {
		return notApplied(op, err)
	}
	if m != "" {
		params["on"] = m
	}
	params["no_parent"] = true
	if tid == "" {
		tid, err = coreFreshID("tb")
		if err != nil {
			return notApplied(op, err)
		}
	}
	labels, _ := params["labels"].(map[string]string)
	if labels == nil {
		labels = map[string]string{}
	}
	labels["herdr.space"] = space
	labels["herdr.tab"] = tid
	labels["where"] = appText(s["name"]) + " / " + cmd.Label
	params["labels"] = labels
	pid, e := coreFreshID("p")
	if e != nil {
		return notApplied(op, e)
	}
	var run coreRunResult
	if err = conn.Call(ctx, "terminal.run", params, &run); err != nil {
		return coreFailure(op, c.endCall(conn, "create", err, true, true))
	}
	if !coreAddressPattern.MatchString(run.Address) || coreMachineOf(run.Address) != m {
		return coreFailure(op, c.endCall(conn, "create", coreProtocol("terminal.run", "invalid terminal address"), true, true))
	}

	local := run.Address
	if m != "" {
		local = strings.TrimPrefix(local, m+"/")
	}
	node := map[string]any{"id": pid, "type": "block", "content": map[string]any{"terminal": local}}
	if split == nil {
		target = map[string]any{"id": tid, "name": cmd.Label, "root": map[string]any{"id": tid, "type": "row", "children": []any{node}}}
		s["tabs"] = append(appArray(s["tabs"]), target)
	} else {
		root, _ := appObject(target["root"])
		if !coreAppSplitNode(root, localTerminal, node, direction, splitID, ratio, 0) {
			return coreFailure(op, &Fault{Code: CodeConflict, Outcome: OutcomeUnknown, Retry: RetryNever})
		}
	}
	if err = coreAppSet(ctx, conn, m, d, obj); err != nil {
		return coreFailure(op, &Fault{Code: CodeConflict, Outcome: OutcomeUnknown, Retry: RetryNever, SafeMessage: "terminal created; App layout changed concurrently; refresh before trying again", Cause: err})
	}
	c.invalidateCoreAppDocs()
	created := []EntityRef{{Kind: EntityPane, ID: encodeCoreAddress(run.Address)}}
	if split == nil {
		created = append(created, EntityRef{Kind: EntityTab, ID: coreAppTabID(m, tid)})
	}
	return Receipt{OperationID: op, Outcome: OutcomeApplied, Created: created}, nil
}
