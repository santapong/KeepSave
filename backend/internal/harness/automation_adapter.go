package harness

import (
	"encoding/json"
	"github.com/santapong/KeepSave/backend/internal/automation"
	"reflect"
)

// ExportAutomation is the composition adapter. Candidate versions and native
// requirements belong to this package, never to the generic run service.
func ExportAutomation(r automation.PackageRequest) (automation.Package, error) {
	p, e := Export(ExportRequest{Harness: r.Client, Endpoint: r.Endpoint, ProfileID: r.ProfileID, ProfileRevision: r.Revision, ProfileDigest: r.ProfileDigest, RequiredControls: r.RequiredControls, SkillName: r.SkillName, SkillSource: r.SkillSource, ArtifactDigest: r.ArtifactDigest})
	if e != nil || p.Manifest.HarnessVersion != r.Version {
		return automation.Package{}, ErrIncompatible
	}
	files := map[string]string{}
	for n, b := range p.Files {
		files[n] = string(b)
	}
	var neutral automation.PackageManifest
	b, e := json.Marshal(p.Manifest)
	if e != nil {
		return automation.Package{}, e
	}
	if e = json.Unmarshal(b, &neutral); e != nil {
		return automation.Package{}, e
	}
	return automation.Package{Harness: r.Client, Version: r.Version, Format: PackageVersion, Files: files, ProfileDigest: r.ProfileDigest, Compatibility: p.Manifest.Controls, Manifest: neutral}, nil
}

// FromAutomation validates a downloaded API package without reading installed
// client settings or claiming that the native client has been qualified.
func FromAutomation(p automation.Package) (Package, error) {
	encoded, e := json.Marshal(p.Manifest)
	if e != nil {
		return Package{}, ErrIncompatible
	}
	var manifest Manifest
	if json.Unmarshal(encoded, &manifest) != nil || p.Harness != manifest.Harness || p.Version != manifest.HarnessVersion || p.Format != PackageVersion || p.ProfileDigest != manifest.ProfileDigest || !reflect.DeepEqual(p.Compatibility, manifest.Controls) {
		return Package{}, ErrIncompatible
	}
	files := map[string][]byte{}
	for name, content := range p.Files {
		files[name] = []byte(content)
	}
	var embedded Manifest
	if json.Unmarshal(files["manifest.json"], &embedded) != nil || !reflect.DeepEqual(manifest, embedded) {
		return Package{}, ErrIncompatible
	}
	result := Package{Files: files, Manifest: manifest}
	if ok, _ := Check(result, manifest.HarnessVersion); !ok {
		return Package{}, ErrIncompatible
	}
	return result, nil
}
