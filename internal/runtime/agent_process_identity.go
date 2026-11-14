package runtime

import (
	"path/filepath"
	"strings"
)

func nestedObservationAgent(argv []string, agent string) bool {
	for _, arg := range argv[:min(2, len(argv))] {
		arg = strings.Trim(arg, "\"'")
		base := filepath.Base(arg)
		if agent == "cmd" && (base == "command-code" || base == "commandcode" || strings.Contains(arg, "/command-code/")) {
			return true
		}
		if base == agent || base == agent+".js" || base == agent+".mjs" || base == agent+"-cli" || strings.Contains(arg, "/node_modules/"+agent+"/") || strings.Contains(arg, "/"+agent+"-cli/") {
			return true
		}
	}
	return false
}
