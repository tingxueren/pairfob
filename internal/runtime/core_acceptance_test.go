package runtime

// Opt-in replay of private, read-only captures through the production Observe
// path, including cold document loading on its independent connection.
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCoreUIAcceptanceReplay(t *testing.T) {
	root := os.Getenv("PAIRFOB_CORE_UI_INPUT")
	if root == "" {
		t.Skip("run scripts/verify-core-ui.sh with private Core captures")
	}
	files, err := filepath.Glob(filepath.Join(root, "captures", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("missing captures: %v", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var capture struct {
				Schema    int                        `json:"schema"`
				Terminals []coreTerminal             `json:"terminals"`
				Docs      map[string]coreAppDocument `json:"docs"`
			}
			if err := json.Unmarshal(raw, &capture); err != nil {
				t.Fatal(err)
			}
			if capture.Schema != 1 {
				t.Fatal("unsupported capture schema")
			}
			core := NewCore("capture-only")
			core.dial = func(context.Context) (coreTransport, coreHello, error) {
				f := newFakeCoreTransport()
				// No core ID: replay must not inspect local native-session bindings.
				f.hello = coreHello{Version: "capture", Generation: 1, Methods: map[string]bool{"terminal.list": true, "doc.get": true}}
				f.handler = func(method string, params json.RawMessage) (any, error) {
					switch method {
					case "terminal.list":
						return map[string]any{"terminals": capture.Terminals}, nil
					case "doc.get":
						var p struct {
							On string `json:"on"`
						}
						if err := json.Unmarshal(params, &p); err != nil {
							return nil, err
						}
						doc, ok := capture.Docs[p.On]
						if !ok {
							return nil, fmt.Errorf("missing document for %q", p.On)
						}
						return doc, nil
					default:
						return nil, fmt.Errorf("capture replay refused %s", method)
					}
				}
				return f, f.hello, nil
			}
			t.Cleanup(func() { _ = core.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := core.Observe(ctx, DefaultSession(), SnapshotQuery{}); err != nil {
				t.Fatal(err)
			}
			waitForCore(t, func() bool {
				core.docMu.Lock()
				defer core.docMu.Unlock()
				return !core.appDocLoading && len(core.appDocs) == len(capture.Docs)
			})
			view, err := core.Observe(ctx, DefaultSession(), SnapshotQuery{})
			if err != nil {
				t.Fatal(err)
			}
			snapshot := view.(SnapshotView).Snapshot
			out, err := json.MarshalIndent(snapshot, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(root, "projected"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "projected", filepath.Base(file)), out, 0600); err != nil {
				t.Fatal(err)
			}
			t.Logf("production Observe: %d workspaces, %d tabs, %d panes", len(snapshot.Workspaces), len(snapshot.Tabs), len(snapshot.Panes))
		})
	}
}
