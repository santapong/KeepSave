package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"github.com/santapong/KeepSave/backend/internal/harness"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: keepsave-harness export --profile JSON --harness codex|hermes --endpoint HTTPS --out DIR | unpack --package JSON --out DIR | check --package DIR --metadata FILE")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	profile := f.String("profile", "", "reviewed portable profile JSON")
	name := f.String("harness", "", "codex or hermes")
	endpoint := f.String("endpoint", "", "canonical HTTPS MCP endpoint")
	out := f.String("out", "", "new output directory")
	packageDir := f.String("package", "", "exported directory for check or downloaded package JSON for unpack")
	metadata := f.String("metadata", "", "explicit package.json or pyproject.toml metadata path; never a provider settings file")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if len(f.Args()) > 0 {
		return errors.New("unexpected arguments")
	}
	switch args[0] {
	case "unpack":
		if *packageDir == "" || *out == "" || *profile != "" || *name != "" || *endpoint != "" || *metadata != "" {
			return errors.New("unpack requires only package JSON and new out directory")
		}
		p, e := unpack(*packageDir, *out)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"unpacked_candidate_package": true, "qualification": "real_harness_acceptance_pending", "native_client_verified": false, "manifest": p.Manifest})
	case "export":
		if *profile == "" || *out == "" {
			return errors.New("profile and out are required")
		}
		b, e := os.ReadFile(*profile)
		if e != nil {
			return e
		}
		var r harness.ExportRequest
		if e = json.Unmarshal(b, &r); e != nil {
			return e
		}
		r.Harness = *name
		r.Endpoint = *endpoint
		p, e := harness.Export(r)
		if e != nil {
			return e
		}
		if e = os.Mkdir(*out, 0700); e != nil {
			return e
		}
		for n, b := range p.Files {
			target := filepath.Join(*out, filepath.FromSlash(n))
			if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
				return e
			}
			if e = os.WriteFile(target, b, 0600); e != nil {
				return e
			}
		}
		return json.NewEncoder(os.Stdout).Encode(p.Manifest)
	case "check":
		if *packageDir == "" || *metadata == "" {
			return errors.New("package and metadata are required")
		}
		files := map[string][]byte{}
		e := filepath.WalkDir(*packageDir, func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.Type()&os.ModeSymlink != 0 {
				return errors.New("package symlinks are not accepted")
			}
			if d.IsDir() {
				return nil
			}
			info, e := d.Info()
			if e != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
				return errors.New("invalid package file")
			}
			relative, e := filepath.Rel(*packageDir, path)
			if e != nil {
				return e
			}
			b, e := os.ReadFile(path)
			if e == nil {
				files[filepath.ToSlash(relative)] = b
			}
			return e
		})
		if e != nil {
			return e
		}
		var m harness.Manifest
		if e = json.Unmarshal(files["manifest.json"], &m); e != nil {
			return e
		}
		version, e := readVersion(*metadata, m.Harness)
		if e != nil {
			return e
		}
		ok, issues := harness.Check(harness.Package{Files: files, Manifest: m}, version)
		result := map[string]any{"compatible_candidate": ok, "qualification": "real_harness_acceptance_pending", "metadata_only": true, "installed_metadata_version": version, "issues": issues}
		if e = json.NewEncoder(os.Stdout).Encode(result); e != nil {
			return e
		}
		if !ok {
			return harness.ErrIncompatible
		}
		return nil
	default:
		return errors.New("unknown command")
	}
}
func readVersion(path, name string) (string, error) {
	expected := "package.json"
	if name == "hermes" {
		expected = "pyproject.toml"
	}
	if filepath.Base(path) != expected {
		return "", errors.New("only explicit harness version metadata is permitted")
	}
	info, e := os.Lstat(path)
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 1<<20 {
		return "", errors.New("only regular bounded harness metadata is permitted")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	if len(b) > 1<<20 {
		return "", errors.New("metadata too large")
	}
	if name == "codex" {
		var p struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}
		if e = json.Unmarshal(b, &p); e != nil {
			return "", e
		}
		if p.Name != "@openai/codex" || p.Version == "" {
			return "", errors.New("not Codex package metadata")
		}
		return p.Version, nil
	}
	if name != "hermes" {
		return "", harness.ErrIncompatible
	}
	var project struct {
		Project struct {
			Name    string `toml:"name"`
			Version string `toml:"version"`
		} `toml:"project"`
	}
	if toml.Unmarshal(b, &project) != nil || project.Project.Name != "hermes-agent" || project.Project.Version == "" {
		return "", errors.New("not Hermes project metadata")
	}
	return project.Project.Version, nil
}
