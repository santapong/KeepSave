package api

import (
	"strings"
	"testing"
)

// FuzzValidateMCPEntryCommand asserts the MCP entry-command validator never
// panics, and that any command it ACCEPTS upholds its core contract: a binary
// with no path separators. This is the authenticated-RCE guard (API-F02/ADR-0010).
func FuzzValidateMCPEntryCommand(f *testing.F) {
	f.Add("node server.js")
	f.Add("/bin/sh -c evil")
	f.Add("node; rm -rf /")
	f.Add("node $(whoami)")
	f.Add("")
	f.Fuzz(func(t *testing.T, entry string) {
		parts, err := validateMCPEntryCommand(entry)
		if err != nil {
			return // rejection is always fine
		}
		if len(parts) == 0 {
			t.Fatalf("accepted %q but returned no argv", entry)
		}
		if strings.ContainsAny(parts[0], `/\`) {
			t.Errorf("accepted entry %q with path separator in binary %q", entry, parts[0])
		}
	})
}
