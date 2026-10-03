package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMetadataOnlyReadsExactHarnessMetadata(t *testing.T) {
	dir := t.TempDir()
	toml := filepath.Join(dir, "pyproject.toml")
	if e := os.WriteFile(toml, []byte("[project]\nname = \"hermes-agent\"\nversion = \"0.21.3\"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	version, e := readVersion(toml, "hermes")
	if e != nil || version != "0.21.3" {
		t.Fatalf("version %s %v", version, e)
	}
	if _, e = readVersion(filepath.Join(dir, "config.yaml"), "hermes"); e == nil {
		t.Fatal("provider settings path accepted")
	}
	json := filepath.Join(dir, "package.json")
	if e = os.WriteFile(json, []byte(`{"name":"other","version":"0.153.3"}`), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = readVersion(json, "codex"); e == nil {
		t.Fatal("wrong package accepted")
	}
	if e = os.Remove(toml); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(json, toml); e != nil {
		t.Fatal(e)
	}
	if _, e = readVersion(toml, "hermes"); e == nil {
		t.Fatal("metadata symlink accepted")
	}
}
