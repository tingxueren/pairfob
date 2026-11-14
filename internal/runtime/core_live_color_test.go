package runtime

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Exercises the adapter against a real PTY, on an explicitly isolated core.
func TestCoreLiveColoredVisibleRead(t *testing.T) {
	coreLiveGated(t, true)
	core := NewCore("")
	t.Cleanup(func() { _ = core.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, hello, err := core.connection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var run coreRunResult
	err = conn.Call(ctx, "terminal.run", map[string]any{
		"argv":      []string{"/bin/sh", "-c", "printf '\033[38;2;137;180;250mcolor_ready中文\033[0m\r\n'; sleep 300"},
		"no_parent": true, "size": map[string]int{"cols": 80, "rows": 24},
	}, &run)
	if err != nil {
		t.Fatal(err)
	}
	pane := encodeCoreAddress(run.Address)
	defer coreLiveClose(t, core, pane)
	var text string
	for {
		view, err := core.Observe(ctx, DefaultSession(), PaneReadQuery{PaneID: pane, Source: SourceVisible, Format: FormatANSI})
		if err != nil {
			t.Fatal(err)
		}
		text = view.(PaneReadView).Text
		if strings.Contains(text, "color_ready中文") {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("fixture not ready")
		case <-time.After(20 * time.Millisecond):
		}
	}
	wantsColor := os.Getenv("PAIRFOB_CORE_LIVE_COLOR") == "1"
	if hello.has("terminal.read_ansi") != wantsColor {
		t.Fatalf("advertised color=%v want=%v", hello.has("terminal.read_ansi"), wantsColor)
	}
	if strings.Contains(text, "38;2;137;180;250") != wantsColor {
		t.Fatalf("color=%v output=%q", wantsColor, text)
	}
	if strings.Contains(text, "?1049") {
		t.Fatal("screen read replayed terminal modes")
	}
	plain, err := core.Observe(ctx, DefaultSession(), PaneReadQuery{PaneID: pane, Source: SourceVisible, Format: FormatText})
	if err != nil || strings.Contains(plain.(PaneReadView).Text, "\x1b") {
		t.Fatalf("plain=%+v err=%v", plain, err)
	}
	t.Logf("real core %s: advertised color=%v, visible text and ANSI verified", hello.Version, wantsColor)
}
