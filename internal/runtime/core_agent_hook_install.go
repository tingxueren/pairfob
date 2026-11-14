package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func installCoreAgentHook(path, binary, agent string) error {
	before, mode, err := hookFile(path)
	if err != nil {
		return err
	}
	settings := map[string]any{}
	if len(before) > 0 {
		d := json.NewDecoder(bytes.NewReader(before))
		d.UseNumber()
		if d.Decode(&settings) != nil || settings == nil {
			return errors.New("agent hooks must be a JSON object")
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			return errors.New("trailing hook data")
		}
	}
	if settings["disableAllHooks"] == true {
		return errors.New("agent hooks are disabled")
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		if settings["hooks"] != nil {
			return errors.New("invalid agent hooks")
		}
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}
	for _, event := range []string{"SessionStart", "SessionEnd"} {
		entries, ok := hooks[event].([]any)
		if hooks[event] != nil && !ok {
			return errors.New("invalid hook group")
		}
		kept := []any{}
		for _, entry := range entries {
			_, ok := entry.(map[string]any)
			if !ok {
				return errors.New("invalid hook entry")
			}

			kept = append(kept, entry)
		}
		command := shellQuoteHook(binary) + " agent-hook " + agent
		// Match only this exact dedicated command on reinstall; no user group is replaced.
		for i := len(kept) - 1; i >= 0; i-- {
			g, _ := kept[i].(map[string]any)
			cs, _ := g["hooks"].([]any)
			if len(cs) == 1 {
				c, _ := cs[0].(map[string]any)
				if c["command"] == command {
					kept = append(kept[:i], kept[i+1:]...)
				}
			}
		}
		timeout := 3
		group := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command, "timeout": timeout}}}
		if event == "SessionStart" {
			group["matcher"] = "*"
		}
		hooks[event] = append(kept, group)
	}
	after, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return replaceHookFile(path, before, append(after, '\n'), mode)
}
