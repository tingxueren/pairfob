package journal

import (
	"testing"
)

func TestExternalHarnessWrappersRemainOutOfChat(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{`<external_codex_apps_open_page>{"page_id":null}</external_codex_apps_open_page>`, ""}, {`<external_codex_apps_selected_page_part_1_of_1>hidden</external_codex_apps_selected_page_part_1_of_1> fix this`, "fix this"}, {`Explain <external_codex_apps_open_page>markup</external_codex_apps_open_page>`, `Explain <external_codex_apps_open_page>markup</external_codex_apps_open_page>`}} {
		if got := visibleCodexUserText(tc.input); got != tc.want {
			t.Fatalf("codex got %q want %q", got, tc.want)
		}
	}
	for _, tag := range []string{"external_system_instructions", "conversation_history"} {
		if got := visibleClaudeUserText("<" + tag + ">hidden</" + tag + "> actual request"); got != "actual request" {
			t.Fatalf("claude %q", got)
		}
	}
}

func TestClaudeExternalWrappersKeepBodyExamples(t *testing.T) {
	for _, input := range []string{"Explain this:\n<conversation_history>user example</conversation_history>", "Example:\n```xml\n<external_system_instructions>example</external_system_instructions>\n```", "<conversation_history>user example without a close"} {
		if got := visibleClaudeUserText(input); got != input {
			t.Fatalf("body altered: %q", got)
		}
	}
	input := "<conversation_history>\nThe conversation so far, each turn of it answered already. Images and files were sent earlier.\nhidden"
	if got := visibleClaudeUserText(input); got != "" {
		t.Fatal("truncated known harness wrapper leaked")
	}
}
