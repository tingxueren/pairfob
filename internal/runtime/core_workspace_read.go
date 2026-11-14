package runtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"pairfob/internal/workspace"
)

// WorkspaceReader is an optional machine-aware read extension. Handled=false
// retains existing local handlers; a remote failure must never fall back locally.
type WorkspaceReader interface {
	ReadRemoteWorkspace(context.Context, SessionRef, string, json.RawMessage) (json.RawMessage, bool, error)
}
type WorkspaceReadHelperRequest struct {
	Version   int             `json:"version"`
	Terminal  string          `json:"terminal"`
	Cwd       string          `json:"cwd"`
	PID       int             `json:"pid"`
	Started   uint64          `json:"started"`
	Operation string          `json:"operation"`
	Params    json.RawMessage `json:"params"`
}
type workspaceReadHelperResponse struct {
	Version int             `json:"version"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

func (c *Core) ReadRemoteWorkspace(ctx context.Context, session SessionRef, op string, raw json.RawMessage) (json.RawMessage, bool, error) {
	var target struct {
		PaneID string `json:"pane_id"`
	}
	if json.Unmarshal(raw, &target) != nil {
		return nil, false, nil
	}
	address, err := decodeCorePaneID(target.PaneID)
	if err != nil || coreMachineOf(address) == "" {
		return nil, false, nil
	}
	if err = coreDefaultSession("workspace", session); err != nil {
		return nil, true, err
	}
	if _, err = workspace.ParseReadRPC(op, raw); err != nil {
		return nil, true, err
	}
	select {
	case coreWorkspaceSlots <- struct{}{}:
		defer func() { <-coreWorkspaceSlots }()
	default:
		return nil, true, &Fault{Code: CodeRateLimited, Outcome: OutcomeNotApplied, Retry: RetryReadSafe, SafeMessage: "too many remote workspace reads"}
	}
	conn, h, err := c.independentConnection(ctx)
	if err != nil {
		return nil, true, err
	}
	defer conn.Close()
	if !h.has("machine.exec") || !h.has("terminal.get") {
		return nil, true, unsupported("workspace", "remote workspace reads unavailable")
	}
	var t coreTerminal
	if err = conn.Call(ctx, "terminal.get", map[string]any{"terminal": address}, &t); err != nil {
		return nil, true, coreFault("workspace", err, false, false)
	}
	if t.Address != address || t.State != "live" || coreResolvedCwd(t) == "" || t.PID <= 1 {
		return nil, true, workspace.ErrNotFound
	}
	machine := coreMachineOf(address)
	local := address[len(machine)+1:]
	input, err := json.Marshal(WorkspaceReadHelperRequest{Version: 1, Terminal: local, Cwd: coreResolvedCwd(t), PID: t.PID, Started: t.ProgramStartedMS, Operation: op, Params: raw})
	if err != nil {
		return nil, true, err
	}
	var result struct {
		Code      *int   `json:"code"`
		Stdout    string `json:"stdout"`
		Truncated bool   `json:"truncated"`
		TimedOut  bool   `json:"timed_out"`
	}
	// The shell text is constant; every client string travels only in stdin JSON.
	params := map[string]any{"on": machine, "argv": []string{"sh", "-c", `exec "$HOME/.local/bin/pairfob" workspace-read-helper`}, "input": base64.StdEncoding.EncodeToString(input), "timeout": 10}
	if err = conn.Call(ctx, "machine.exec", params, &result); err != nil {
		return nil, true, coreFault("workspace", err, false, false)
	}
	if result.Code == nil || *result.Code != 0 || result.Truncated || result.TimedOut || len(result.Stdout) > 2<<20 {
		return nil, true, unsupported("workspace", "target needs the fork workspace helper")
	}
	data, err := base64.StdEncoding.DecodeString(result.Stdout)
	if err != nil {
		return nil, true, coreProtocol("workspace", "invalid helper response")
	}
	var reply workspaceReadHelperResponse
	if json.Unmarshal(data, &reply) != nil || reply.Version != 1 {
		return nil, true, unsupported("workspace", "target workspace helper is incompatible")
	}
	switch reply.Error {
	case "":
		if !json.Valid(reply.Result) {
			return nil, true, coreProtocol("workspace", "missing helper result")
		}
		return reply.Result, true, nil
	case "invalid_path":
		return nil, true, workspace.ErrInvalidPath
	case "changed":
		return nil, true, workspace.ErrChanged
	case "not_found":
		return nil, true, workspace.ErrNotFound
	case "not_repository":
		return nil, true, workspace.ErrNotRepository
	default:
		return nil, true, errors.New("remote workspace inspection failed")
	}
}
func (c *Core) RunWorkspaceReadHelper(ctx context.Context, input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(io.LimitReader(input, 65537))
	if err != nil || len(data) > 65536 {
		return errors.New("invalid helper input")
	}
	var request WorkspaceReadHelperRequest
	if json.Unmarshal(data, &request) != nil || request.Version != 1 || !coreAddressPartPattern.MatchString(request.Terminal) || request.PID <= 1 {
		return errors.New("invalid helper request")
	}
	c.opMu.Lock()
	defer c.opMu.Unlock()
	conn, h, err := c.connection(ctx)
	if err != nil {
		return err
	}
	if !h.has("terminal.get") {
		return errors.New("terminal lookup unavailable")
	}
	check := func() bool {
		var t coreTerminal
		if conn.Call(ctx, "terminal.get", map[string]any{"terminal": request.Terminal}, &t) != nil {
			return false
		}
		return t.Address == request.Terminal && t.State == "live" && t.PID == request.PID && t.ProgramStartedMS == request.Started && coreResolvedCwd(t) == request.Cwd
	}
	var result any
	if !check() {
		err = workspace.ErrChanged
	} else {
		result, err = workspace.ReadRPC(request.Operation, request.Params, request.Cwd)
		if !check() {
			result = nil
			err = workspace.ErrChanged
		}
	}
	reply := workspaceReadHelperResponse{Version: 1}
	if err != nil {
		switch {
		case errors.Is(err, workspace.ErrInvalidPath):
			reply.Error = "invalid_path"
		case errors.Is(err, workspace.ErrChanged):
			reply.Error = "changed"
		case errors.Is(err, workspace.ErrNotFound), errors.Is(err, workspace.ErrNotDirectory):
			reply.Error = "not_found"
		case errors.Is(err, workspace.ErrNotRepository):
			reply.Error = "not_repository"
		default:
			reply.Error = "internal"
		}
	} else {
		reply.Result, err = json.Marshal(result)
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(output).Encode(reply)
}

var coreWorkspaceSlots = make(chan struct{}, 4)

func (c *Core) independentConnection(ctx context.Context) (coreTransport, coreHello, error) {
	c.connMu.Lock()
	closed := c.closed
	c.connMu.Unlock()
	if closed {
		return nil, coreHello{}, coreClosedFault("connect")
	}
	conn, h, err := c.dialer()(ctx)
	if err != nil {
		return nil, coreHello{}, coreFault("connect", err, false, false)
	}
	return conn, h, nil
}
