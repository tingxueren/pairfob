package runtime

import (
	"context"
	"errors"
	"time"
)

func (c *Core) Observe(ctx context.Context, session SessionRef, query Query) (View, error) {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	if err := coreDefaultSession("observe", session); err != nil {
		return nil, err
	}
	switch value := query.(type) {
	case SnapshotQuery:
		return c.coreSnapshot(ctx)
	case PaneReadQuery:
		return c.coreReadPane(ctx, value)
	case HistoryQuery:
		return nil, unsupported("history", "the terminal core has no transcript history")
	default:
		return nil, unsupported("observe", "unknown runtime query")
	}
}

func (c *Core) coreSnapshot(ctx context.Context) (View, error) {
	conn, hello, err := c.connection(ctx)
	if err != nil {
		return nil, err
	}
	if !hello.has("terminal.list") {
		return nil, unsupported("snapshot", "snapshot is unsupported by the terminal core")
	}
	var list coreListResult
	if err := conn.Call(ctx, "terminal.list", map[string]any{"all": false}, &list); err != nil {
		return nil, c.endCall(conn, "snapshot", err, false, false)
	}
	if list.Terminals == nil {
		return nil, c.endCall(conn, "snapshot", coreProtocol("terminal.list", "terminal list is missing"), false, false)
	}
	terminals, err := coreLiveTerminals(*list.Terminals)
	if err != nil {
		return nil, c.endCall(conn, "snapshot", err, false, false)
	}
	snapshot := c.coreAppSnapshot(ctx, conn, hello, terminals, time.Now().Unix())
	c.applyCoreAgentBindings(ctx, hello, terminals, &snapshot)
	return SnapshotView{Snapshot: snapshot}, nil
}

func (c *Core) coreReadPane(ctx context.Context, query PaneReadQuery) (View, error) {
	if query.PaneID == "" {
		return nil, invalidFault("pane.read", "pane id is required")
	}
	terminal, err := decodeCorePaneID(query.PaneID)
	if err != nil {
		return nil, err
	}
	if query.Source == "" {
		query.Source = SourceVisible
	}
	if query.Format == "" {
		query.Format = FormatText
	}
	if !validPaneReadSource(query.Source) || (query.Format != FormatText && query.Format != FormatANSI) {
		return nil, invalidFault("pane.read", "invalid pane read options")
	}
	lines := query.Lines
	if query.Source == SourceVisible {
		lines = 0
	}
	if lines < 0 {
		lines = 0
	}
	if lines > 4096 {
		lines = 4096
	}
	conn, hello, err := c.connection(ctx)
	if err != nil {
		return nil, err
	}
	if !hello.has("terminal.read") {
		return nil, unsupported("pane.read", "pane read is unsupported by the terminal core")
	}
	method := "terminal.read"
	params := map[string]any{"terminal": terminal, "lines": lines}
	if query.Source == SourceVisible && query.Format == FormatANSI && hello.has("terminal.read_ansi") {
		method = "terminal.read_ansi"
		params = map[string]any{"terminal": terminal}
	}
	var read coreReadResult
	readErr := conn.Call(ctx, method, params, &read)
	var rpcErr *coreRPCError
	// A local hello does not promise the target remote core has this optional
	// method. Only explicit unsupported replies justify a plain-screen read.
	if method == "terminal.read_ansi" && errors.As(readErr, &rpcErr) && (rpcErr.Kind == "unknown_method" || rpcErr.Kind == "needs_update") {
		method = "terminal.read"
		read = coreReadResult{}
		readErr = conn.Call(ctx, method, map[string]any{"terminal": terminal, "lines": 0}, &read)
	}
	if readErr != nil {
		return nil, c.endCall(conn, "pane.read", readErr, false, false)
	}
	if (method == "terminal.read_ansi" && read.How != "screen") || (read.How != "screen" && read.How != "history" && read.How != "stitched") {
		return nil, c.endCall(conn, "pane.read", coreProtocol(method, "invalid terminal read"), false, false)
	}
	// Older cores expose only text reads; retain that degradation unless the
	// server explicitly advertises the separate colored-screen method.
	return PaneReadView{Text: read.Text, Truncated: !read.Complete}, nil
}
