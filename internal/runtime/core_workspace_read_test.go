package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreRemoteWorkspaceRouteAndMissingHelper(t *testing.T) {
	f := newFakeCoreTransport()
	f.hello.Methods["machine.exec"] = true
	f.handler = func(m string, p json.RawMessage) (any, error) {
		switch m {
		case "terminal.get":
			return coreTerminal{Address: "dev/t3", State: "live", Cwd: "/remote/repo", PID: 100, ProgramStartedMS: 1}, nil
		case "machine.exec":
			var q struct {
				On    string   `json:"on"`
				Argv  []string `json:"argv"`
				Input string   `json:"input"`
			}
			if json.Unmarshal(p, &q) != nil || q.On != "dev" || !strings.Contains(strings.Join(q.Argv, " "), "workspace-read-helper") {
				t.Fatal("route", string(p))
			}
			data, _ := base64.StdEncoding.DecodeString(q.Input)
			var r WorkspaceReadHelperRequest
			if json.Unmarshal(data, &r) != nil || r.Terminal != "t3" || r.Cwd != "/remote/repo" || r.PID != 100 {
				t.Fatal("authority", string(data))
			}
			return map[string]any{"code": 0, "stdout": base64.StdEncoding.EncodeToString([]byte(`{"version":1,"result":{"content":"remote"}}`))}, nil
		}
		return coreFakeDefault(m, p)
	}
	c := newTestCore(t, f)
	raw := json.RawMessage(`{"pane_id":"dev:t3","path":"note.txt"}`)
	result, handled, err := c.ReadRemoteWorkspace(context.Background(), DefaultSession(), "WorkspaceRead", raw)
	if !handled || err != nil || !bytes.Contains(result, []byte("remote")) {
		t.Fatal(string(result), handled, err)
	}
	base := f.handler
	f.handler = func(m string, p json.RawMessage) (any, error) {
		if m == "machine.exec" {
			return map[string]any{"code": 127, "stdout": ""}, nil
		}
		return base(m, p)
	}
	_, handled, err = c.ReadRemoteWorkspace(context.Background(), DefaultSession(), "WorkspaceRead", raw)
	if !handled || err == nil {
		t.Fatal("missing helper fell back")
	}
	_, handled, err = c.ReadRemoteWorkspace(context.Background(), DefaultSession(), "WorkspaceRead", json.RawMessage(`{"pane_id":"t3","path":"note.txt"}`))
	if handled || err != nil {
		t.Fatal("local path intercepted")
	}
}
func TestCoreWorkspaceHelperRootAuthority(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(root, "note.txt"), []byte("remote"), 0600)
	os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0600)
	os.Symlink(outside, filepath.Join(root, "escape"))
	for _, tc := range []struct {
		name, path, cwd string
		changed         bool
		want            string
	}{{"read", "note.txt", root, false, ""}, {"escape", "escape/secret", root, false, "invalid_path"}, {"wrongcwd", "note.txt", outside, false, "changed"}, {"switchduringread", "note.txt", root, true, "changed"}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeCoreTransport()
			calls := 0
			f.handler = func(m string, p json.RawMessage) (any, error) {
				if m == "terminal.get" {
					calls++
					cwd := root
					if tc.changed && calls > 1 {
						cwd = outside
					}
					return coreTerminal{Address: "t3", State: "live", Cwd: cwd, PID: 100, ProgramStartedMS: 1}, nil
				}
				return coreFakeDefault(m, p)
			}
			q := WorkspaceReadHelperRequest{Version: 1, Terminal: "t3", Cwd: tc.cwd, PID: 100, Started: 1, Operation: "WorkspaceRead", Params: json.RawMessage(`{"pane_id":"dev:t3","path":"` + tc.path + `"}`)}
			data, _ := json.Marshal(q)
			var output bytes.Buffer
			err := newTestCore(t, f).RunWorkspaceReadHelper(context.Background(), bytes.NewReader(data), &output)
			if err != nil {
				t.Fatal(err)
			}
			var reply workspaceReadHelperResponse
			if json.Unmarshal(output.Bytes(), &reply) != nil || reply.Error != tc.want {
				t.Fatal(output.String())
			}
			if tc.want != "" && len(reply.Result) > 0 {
				t.Fatal("failed read leaked result")
			}
		})
	}
}
