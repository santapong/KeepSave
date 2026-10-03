package automation_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	. "github.com/santapong/KeepSave/backend/internal/automation"
	"github.com/santapong/KeepSave/backend/internal/harness"
)

const testSource = "---\nname: approved-review\ndescription: Review a bound repository.\n---\nUse repository_tree and read_file only for the approved run.\n"

func TestStrictInstructionSourceAndDigest(t *testing.T) {
	if ValidateSource("approved-review", testSource) != nil {
		t.Fatal("portable instructions denied")
	}
	if Digest([]byte("abc")) != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" || Digest([]byte(testSource)) == Digest([]byte(testSource+"change")) {
		t.Fatal("source digest unstable")
	}
	for _, source := range []string{strings.Replace(testSource, "name: approved-review", "name: another", 1), strings.Replace(testSource, "description:", "name: approved-review\ndescription:", 1), strings.Replace(testSource, "description:", "description: duplicate\ndescription:", 1), strings.Replace(testSource, "description: Review a bound repository.", "description: ", 1), strings.Replace(testSource, "description: Review a bound repository.", "description: "+strings.Repeat("x", 1025), 1), strings.Replace(testSource, "description:", "allowed-tools: shell\ndescription:", 1), strings.Replace(testSource, "description:", "dependencies: [curl]\ndescription:", 1), strings.Replace(testSource, "description:", "extension: true\ndescription:", 1), "plain instructions", testSource + "\x00", testSource + string([]byte{0xff}), testSource + strings.Repeat("x", 64<<10), "---\nname: approved-review\ndescription: blank\n---\n"} {
		if ValidateSource("approved-review", source) == nil {
			t.Fatal("nonportable source accepted")
		}
	}
	for _, name := range []string{"../review", "UPPER", "bad_name", strings.Repeat("a", 65)} {
		if ValidateSource(name, testSource) == nil {
			t.Fatal("unsafe skill name accepted")
		}
	}
}

func TestManifestBoundsAndExactOperations(t *testing.T) {
	base := NewManifest(Digest([]byte(testSource)))
	expected, e := base.Canonical()
	if e != nil {
		t.Fatal(e)
	}
	again, _ := base.Canonical()
	if !bytes.Equal(expected, again) {
		t.Fatal("canonical bytes unstable")
	}
	cases := map[string]func(*Manifest){"catalog": func(m *Manifest) { m.CatalogDigest = strings.Repeat("e", 64) }, "digest": func(m *Manifest) { m.ArtifactDigest = "bad" }, "seconds": func(m *Manifest) { m.MaxSeconds = 601 }, "seconds_zero": func(m *Manifest) { m.MaxSeconds = 0 }, "tool": func(m *Manifest) { m.ToolSeconds = 31 }, "tool_zero": func(m *Manifest) { m.ToolSeconds = 0 }, "response": func(m *Manifest) { m.MaxResponseBytes = 4<<20 + 1 }, "response_low": func(m *Manifest) { m.MaxResponseBytes = 1 }, "provider": func(m *Manifest) { m.MaxProviderOperations = 101 }, "provider_zero": func(m *Manifest) { m.MaxProviderOperations = 0 }, "bytes": func(m *Manifest) { m.MaxTotalBytes = 32<<20 + 1 }, "bytes_zero": func(m *Manifest) { m.MaxTotalBytes = 0 }, "concurrent": func(m *Manifest) { m.MaxConcurrent = 3 }, "concurrent_zero": func(m *Manifest) { m.MaxConcurrent = 0 }, "controls": func(m *Manifest) { m.Controls = "client_reported" }, "operations": func(m *Manifest) { m.Operations = []string{"repository_tree", "shell"} }, "extra": func(m *Manifest) { m.Operations = append(m.Operations, "read_file") }, "reorder": func(m *Manifest) { m.Operations = []string{"read_file", "repository_tree"} }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			m := NewManifest(base.ArtifactDigest)
			change(&m)
			if _, e := m.Canonical(); e == nil {
				t.Fatal("unsupported authority bound accepted")
			}
		})
	}
}

func TestNativePackagesPreserveSourceAndPendingQualification(t *testing.T) {
	digest := Digest([]byte("approved immutable profile"))
	options := PackageOptions{Endpoint: "https://control.example/mcp", ProfileID: "reviewed-profile", Revision: 1, Exporter: harness.ExportAutomation}
	for _, candidate := range []struct{ client, version, sourcePath string }{{"codex", "0.153.3", ".agents/skills/approved-review/SKILL.md"}, {"hermes", "0.21.5", "skills/approved-review/SKILL.md"}} {
		t.Run(candidate.client, func(t *testing.T) {
			p, e := NativePackage(candidate.client, candidate.version, "approved-review", testSource, digest, options)
			if e != nil {
				t.Fatal(e)
			}
			if p.Files[candidate.sourcePath] != testSource || p.ProfileDigest != digest || p.Manifest.ArtifactDigest != Digest([]byte(testSource)) || p.Manifest.HarnessVersion != candidate.version || p.Manifest.Qualification != "real_harness_acceptance_pending" {
				t.Fatal("source/identity or qualification changed")
			}
			for file, want := range p.Manifest.FileDigests {
				if Digest([]byte(p.Files[file])) != want {
					t.Fatal("file digest mismatch")
				}
			}
			var manifest map[string]any
			if json.Unmarshal([]byte(p.Files["manifest.json"]), &manifest) != nil || manifest["qualification"] != "real_harness_acceptance_pending" {
				t.Fatal("manifest falsely qualified")
			}
			if p.Compatibility["native_stop"] != "unverified" || p.Compatibility["native_skill_immutability"] != "unavailable" {
				t.Fatal("native controls falsely enforced")
			}
			if candidate.client == "hermes" && !strings.Contains(p.Files["config.yaml"], "retain the chosen non-Anthropic provider") {
				t.Fatal("Hermes provider preference lost")
			}
			for file := range p.Files {
				if strings.HasPrefix(file, "scripts/") {
					t.Fatal("executable dependency packaged")
				}
			}
		})
	}
}

func TestNativePackageRefusesUnsupportedCandidateAndRequiredControls(t *testing.T) {
	digest := Digest([]byte("profile"))
	options := PackageOptions{Endpoint: "https://control.example/mcp", ProfileID: "reviewed-profile", Revision: 1, Exporter: harness.ExportAutomation}
	for _, candidate := range []struct{ client, version string }{{"codex", "0.153.2"}, {"hermes", "0.21.3"}, {"unknown", "1"}} {
		if _, e := NativePackage(candidate.client, candidate.version, "approved-review", testSource, digest, options); e == nil {
			t.Fatal("unreviewed candidate accepted")
		}
	}
	if _, e := NativePackage("codex", "0.153.3", "approved-review", testSource, digest); e == nil {
		t.Fatal("missing profile options accepted")
	}
	options.RequiredControls = []string{"native_stop"}
	if _, e := NativePackage("hermes", "0.21.5", "approved-review", testSource, digest, options); e == nil {
		t.Fatal("unverified required control downgraded")
	}
	options.RequiredControls = nil
	for _, endpoint := range []string{"http://control.example/mcp", "https://control.example/other", "https://user:pass@control.example/mcp"} {
		options.Endpoint = endpoint
		if _, e := NativePackage("codex", "0.153.3", "approved-review", testSource, digest, options); e == nil {
			t.Fatal("unsafe endpoint packaged")
		}
	}
}
