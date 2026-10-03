package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santapong/KeepSave/backend/internal/automation"
	"github.com/santapong/KeepSave/backend/internal/harness"
)

func downloadedCandidate(t *testing.T, name string) automation.Package {
	t.Helper()
	version := "0.21.5"
	if name == "codex" {
		version = "0.153.3"
	}
	source := "---\nname: safe-review\ndescription: Review the approved repository.\n---\nUse only approved bounded repository reads.\n"
	p, err := automation.NativePackage(name, version, "safe-review", source, strings.Repeat("a", 64), automation.PackageOptions{Endpoint: "https://app.keepsave.example/mcp", ProfileID: "reviewed-profile", Revision: 1, Exporter: harness.ExportAutomation})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeDownloadedCandidate(t *testing.T, dir string, p automation.Package) string {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "downloaded-package.json")
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return input
}

func TestUnpackDownloadedNativeCandidates(t *testing.T) {
	for _, name := range []string{"codex", "hermes"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			original := downloadedCandidate(t, name)
			input := writeDownloadedCandidate(t, dir, original)
			output := filepath.Join(dir, "isolated-candidate")
			p, err := unpack(input, output)
			if err != nil {
				t.Fatal(err)
			}
			if ok, issues := harness.Check(p, original.Version); !ok {
				t.Fatal("downloaded package lost candidate identity", issues)
			}
			if p.Manifest.Qualification != "real_harness_acceptance_pending" {
				t.Fatal("unpacking attested native acceptance")
			}
			files := map[string]string{}
			if err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if info.Mode().Perm()&0077 != 0 {
						t.Fatal("candidate directory is not private")
					}
					return nil
				}
				if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
					t.Fatal("candidate file is not private and regular")
				}
				relative, err := filepath.Rel(output, path)
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(path)
				files[filepath.ToSlash(relative)] = string(raw)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			if len(files) != len(original.Files) {
				t.Fatal("unpack changed file set")
			}
			for file, expected := range original.Files {
				if files[file] != expected {
					t.Fatal("unpack changed reviewed content", file)
				}
			}
		})
	}
}

func TestUnpackRefusesTamperingBeforeCreatingDirectory(t *testing.T) {
	for _, change := range []string{"file", "missing_file", "file_digest", "tree_digest", "outer_manifest", "identity", "compatibility", "unknown", "duplicate", "trailing"} {
		t.Run(change, func(t *testing.T) {
			dir := t.TempDir()
			p := downloadedCandidate(t, "hermes")
			switch change {
			case "file":
				p.Files["config.yaml"] += "unreviewed: true\n"
			case "missing_file":
				delete(p.Files, "IMPORT.md")
			case "file_digest", "tree_digest":
				// Alter both representations to test a self-consistent manifest,
				// rather than only the outer/embedded identity comparison.
				if change == "file_digest" {
					p.Manifest.FileDigests["config.yaml"] = strings.Repeat("0", 64)
				} else {
					p.Manifest.TreeDigest = strings.Repeat("0", 64)
				}
				manifest, err := json.MarshalIndent(p.Manifest, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				p.Files["manifest.json"] = string(append(manifest, '\n'))
			case "outer_manifest":
				p.Manifest.Qualification = "accepted"
			case "identity":
				p.Version = "0.21.3"
			case "compatibility":
				p.Compatibility["credential_custody"] = "native"
			}
			raw, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "unknown":
				raw = []byte(strings.TrimSuffix(string(raw), "}") + `,"unknown":true}`)
			case "duplicate":
				raw = []byte(strings.Replace(string(raw), `"harness":"hermes"`, `"harness":"hermes","harness":"hermes"`, 1))
			case "trailing":
				raw = append(raw, []byte(` {"second":true}`)...)
			}
			input := filepath.Join(dir, "package.json")
			if err := os.WriteFile(input, raw, 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "refused")
			if _, err := unpack(input, output); err == nil {
				t.Fatal("unreviewed package unpacked")
			}
			if _, err := os.Lstat(output); !os.IsNotExist(err) {
				t.Fatal("refused package created output")
			}
		})
	}
}

func TestUnpackRejectsTraversalAndSizeLimits(t *testing.T) {
	for _, file := range []string{"../escape", "/escape", `skills\escape`, "C:/escape", "."} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			p := downloadedCandidate(t, "hermes")
			p.Files[file] = "escape"
			if _, err := unpack(writeDownloadedCandidate(t, dir, p), filepath.Join(dir, "refused")); err == nil {
				t.Fatal("traversal file accepted")
			}
		})
	}
	for _, limit := range []string{"input", "count", "file", "total"} {
		t.Run(limit, func(t *testing.T) {
			dir := t.TempDir()
			p := downloadedCandidate(t, "hermes")
			switch limit {
			case "count":
				for i := 0; i <= maxPackageFiles; i++ {
					p.Files["extra/"+strings.Repeat("a", i+1)] = "x"
				}
			case "file":
				p.Files["config.yaml"] = strings.Repeat("a", maxPackageFile+1)
			case "total":
				p.Files["extra/a"] = strings.Repeat("a", maxPackageFile)
				p.Files["extra/b"] = strings.Repeat("a", maxPackageFile)
			}
			input := writeDownloadedCandidate(t, dir, p)
			if limit == "input" {
				if err := os.Truncate(input, maxPackageJSON+1); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := unpack(input, filepath.Join(dir, "refused")); err == nil {
				t.Fatal("package limit accepted")
			}
		})
	}
}

func TestUnpackRefusesSymlinksAndExistingOutput(t *testing.T) {
	dir := t.TempDir()
	input := writeDownloadedCandidate(t, dir, downloadedCandidate(t, "hermes"))
	inputLink := filepath.Join(dir, "input-link.json")
	if err := os.Symlink(input, inputLink); err != nil {
		t.Fatal(err)
	}
	if _, err := unpack(inputLink, filepath.Join(dir, "refused")); err == nil {
		t.Fatal("input symlink accepted")
	}
	inputParent := filepath.Join(dir, "input-parent-link")
	if err := os.Symlink(dir, inputParent); err != nil {
		t.Fatal(err)
	}
	if _, err := unpack(filepath.Join(inputParent, filepath.Base(input)), filepath.Join(dir, "refused")); err == nil {
		t.Fatal("input ancestor symlink accepted")
	}
	existing := filepath.Join(dir, "existing")
	if err := os.Mkdir(existing, 0700); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(existing, "keep")
	if err := os.WriteFile(canary, []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := unpack(input, existing); err == nil {
		t.Fatal("existing output overwritten")
	}
	link := filepath.Join(dir, "linked-directory")
	if err := os.Symlink(existing, link); err != nil {
		t.Fatal(err)
	}
	if _, err := unpack(input, filepath.Join(link, "candidate")); err == nil {
		t.Fatal("symbolic output ancestor accepted")
	}
	if _, err := unpack(input, link); err == nil {
		t.Fatal("symbolic output leaf accepted")
	}
	if raw, err := os.ReadFile(canary); err != nil || string(raw) != "preserved" {
		t.Fatal("existing profile changed", err)
	}
}

func TestCandidateDirectoryHandleCannotBeRedirectedByAncestorReplacement(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "parent")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := candidateDirectoryRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	moved := filepath.Join(dir, "original-parent")
	if err := os.Rename(parent, moved); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(dir, "replacement")
	if err := os.Mkdir(replacement, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(replacement, parent); err != nil {
		t.Fatal(err)
	}
	if err := root.Mkdir("candidate", 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(moved, "candidate")); err != nil {
		t.Fatal("pinned directory was lost", err)
	}
	if _, err := os.Lstat(filepath.Join(replacement, "candidate")); !os.IsNotExist(err) {
		t.Fatal("substituted ancestor received a write")
	}
	if substituted, err := candidateDirectoryRoot(parent); err == nil {
		substituted.Close()
		t.Fatal("new handle accepted a substituted symbolic ancestor")
	}
}

func TestUnpackCommandRequiresOnlyDownloadedPackageAndNewDirectory(t *testing.T) {
	dir := t.TempDir()
	input := writeDownloadedCandidate(t, dir, downloadedCandidate(t, "hermes"))
	if err := run([]string{"unpack", "--package", input, "--out", filepath.Join(dir, "candidate")}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"unpack", "--package", input, "--out", filepath.Join(dir, "refused"), "--metadata", "/unused/provider/settings"}); err == nil {
		t.Fatal("unpack accepted native metadata operation")
	}
}
