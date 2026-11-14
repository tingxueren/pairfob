package runtime

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCoreVisibleANSIUsesAdvertisedReadWithoutSendingIgnoredFields(t *testing.T) {
	fake := newFakeCoreTransport()
	fake.hello.Methods["terminal.read_ansi"] = true
	fake.handler = func(method string, params json.RawMessage) (any, error) {
		if method == "terminal.read_ansi" {
			if string(params) != `{"terminal":"t3"}` {
				t.Fatalf("read_ansi params=%s", params)
			}
			return map[string]any{"text": "\x1b[31mred\x1b[0m", "complete": true, "how": "screen"}, nil
		}
		return coreFakeDefault(method, params)
	}
	core := newTestCore(t, fake)
	view, err := core.Observe(context.Background(), DefaultSession(), PaneReadQuery{PaneID: "t3", Source: SourceVisible, Format: FormatANSI})
	if err != nil || view.(PaneReadView).Text != "\x1b[31mred\x1b[0m" {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	if len(fake.callsOf("terminal.read")) != 0 {
		t.Fatal("ANSI screen fell back to plain text")
	}
}

func TestCoreColoredReadKeepsTextAndHistoryOnOriginalMethod(t *testing.T) {
	fake := newFakeCoreTransport()
	fake.hello.Methods["terminal.read_ansi"] = true
	core := newTestCore(t, fake)
	for _, query := range []PaneReadQuery{
		{PaneID: "t3", Source: SourceVisible, Format: FormatText},
		{PaneID: "t3", Source: SourceRecent, Format: FormatANSI, Lines: 200},
	} {
		if _, err := core.Observe(context.Background(), DefaultSession(), query); err != nil {
			t.Fatal(err)
		}
	}
	if len(fake.callsOf("terminal.read")) != 2 || len(fake.callsOf("terminal.read_ansi")) != 0 {
		t.Fatal("text/history changed method")
	}
}

func TestCoreColoredReadFailureDoesNotSilentlyFallBack(t *testing.T) {
	for _, test := range []struct {
		name   string
		result any
		err    error
	}{
		{name: "unavailable", err: &coreRPCError{Kind: "unavailable", Message: "not ready"}},

		{name: "history response", result: map[string]any{"text": "red", "complete": true, "how": "history"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeCoreTransport()
			fake.hello.Methods["terminal.read_ansi"] = true
			fake.handler = func(method string, params json.RawMessage) (any, error) {
				if method == "terminal.read_ansi" {
					return test.result, test.err
				}
				return coreFakeDefault(method, params)
			}
			core := newTestCore(t, fake)
			_, err := core.Observe(context.Background(), DefaultSession(), PaneReadQuery{PaneID: "t3", Source: SourceVisible, Format: FormatANSI})
			if err == nil || len(fake.callsOf("terminal.read")) != 0 {
				t.Fatalf("err=%v plain calls=%d", err, len(fake.callsOf("terminal.read")))
			}
		})
	}
}

func TestCoreColoredReadFallsBackOnlyForExplicitUnsupportedMethod(t *testing.T) {
	for _, kind := range []string{"unknown_method", "needs_update"} {
		t.Run(kind, func(t *testing.T) {
			fake := newFakeCoreTransport()
			fake.hello.Methods["terminal.read_ansi"] = true
			fake.handler = func(method string, params json.RawMessage) (any, error) {
				if method == "terminal.read_ansi" {
					return nil, &coreRPCError{Kind: kind, Message: "remote core needs update"}
				}
				if method == "terminal.read" {
					if string(params) != `{"lines":0,"terminal":"devbox/t7"}` {
						t.Fatalf("fallback params=%s", params)
					}
					return map[string]any{"text": "old remote ready", "complete": true, "how": "screen"}, nil
				}
				return coreFakeDefault(method, params)
			}
			core := newTestCore(t, fake)
			view, err := core.Observe(context.Background(), DefaultSession(), PaneReadQuery{PaneID: "devbox:t7", Source: SourceVisible, Format: FormatANSI})
			if err != nil || view.(PaneReadView).Text != "old remote ready" {
				t.Fatalf("view=%+v err=%v", view, err)
			}
			if len(fake.callsOf("terminal.read_ansi")) != 1 || len(fake.callsOf("terminal.read")) != 1 {
				t.Fatal("fallback must read exactly once")
			}
		})
	}
}
