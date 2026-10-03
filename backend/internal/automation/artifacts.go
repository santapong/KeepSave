// Package automation validates immutable instruction-only sources and profiles.
// Native rendering delegates to the reviewed harness adapters. Packaging never
// attests that a developer's device enforced a profile.
package automation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/santapong/KeepSave/backend/internal/mcpgateway/catalog"
	"gopkg.in/yaml.v3"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid instruction artifact")
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func Digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func ValidateSource(name, source string) error {
	if !namePattern.MatchString(name) || !utf8.ValidString(source) || len(source) > 64<<10 || strings.ContainsRune(source, 0) || !strings.HasPrefix(source, "---\n") {
		return ErrInvalid
	}
	parts := strings.SplitN(strings.TrimPrefix(source, "---\n"), "\n---\n", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return ErrInvalid
	}
	var front struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	d := yaml.NewDecoder(strings.NewReader(parts[0]))
	d.KnownFields(true)
	if d.Decode(&front) != nil || front.Name != name || strings.TrimSpace(front.Description) == "" || len(front.Description) > 1024 {
		return ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return ErrInvalid
	}
	return nil
}
func CatalogDigest() string { b, _ := json.Marshal(catalog.Definitions()); return Digest(b) }

type Manifest struct {
	CatalogDigest         string   `json:"catalog_digest"`
	ArtifactDigest        string   `json:"artifact_digest"`
	Operations            []string `json:"operations"`
	MaxSeconds            int      `json:"max_seconds"`
	ToolSeconds           int      `json:"tool_seconds"`
	MaxResponseBytes      int      `json:"max_response_bytes"`
	MaxProviderOperations int      `json:"max_provider_operations"`
	MaxTotalBytes         int64    `json:"max_total_bytes"`
	MaxConcurrent         int      `json:"max_concurrent"`
	Controls              string   `json:"controls"`
}

func NewManifest(digest string) Manifest {
	return Manifest{CatalogDigest(), digest, []string{"repository_tree", "read_file"}, 600, 30, 4 << 20, 100, 32 << 20, 2, "server_enforced"}
}
func (m Manifest) Canonical() ([]byte, error) {
	if m.CatalogDigest != CatalogDigest() || !digestPattern.MatchString(m.ArtifactDigest) || m.MaxSeconds < 1 || m.MaxSeconds > 600 || m.ToolSeconds < 1 || m.ToolSeconds > 30 || m.MaxResponseBytes < 128<<10 || m.MaxResponseBytes > 4<<20 || m.MaxProviderOperations < 1 || m.MaxProviderOperations > 100 || m.MaxTotalBytes < 1 || m.MaxTotalBytes > 32<<20 || m.MaxConcurrent < 1 || m.MaxConcurrent > 2 || m.Controls != "server_enforced" || len(m.Operations) != 2 || m.Operations[0] != "repository_tree" || m.Operations[1] != "read_file" {
		return nil, ErrInvalid
	}
	return json.Marshal(m)
}

type Package struct {
	Harness       string            `json:"harness"`
	Version       string            `json:"version"`
	Format        string            `json:"format"`
	Files         map[string]string `json:"files"`
	ProfileDigest string            `json:"profile_digest"`
	Compatibility map[string]string `json:"compatibility"`
	Manifest      PackageManifest   `json:"manifest"`
}

// PackageManifest records adapter qualification independently of server authority.
type PackageManifest struct {
	SchemaVersion    string            `json:"schema_version"`
	PackageVersion   string            `json:"package_version"`
	Harness          string            `json:"harness"`
	HarnessVersion   string            `json:"harness_version"`
	SDKVersion       string            `json:"sdk_version"`
	Protocol         string            `json:"protocol"`
	OS               string            `json:"os"`
	Endpoint         string            `json:"endpoint"`
	ClientID         string            `json:"client_id"`
	Callback         string            `json:"callback"`
	ProfileID        string            `json:"profile_id"`
	ProfileRevision  int64             `json:"profile_revision"`
	ProfileDigest    string            `json:"profile_digest"`
	SkillName        string            `json:"skill_name"`
	ArtifactDigest   string            `json:"artifact_digest"`
	RequiredControls []string          `json:"required_controls"`
	Controls         map[string]string `json:"controls"`
	FileDigests      map[string]string `json:"file_digests"`
	TreeDigest       string            `json:"tree_digest"`
	Qualification    string            `json:"qualification"`
	SourceCheckedAt  string            `json:"source_checked_at"`
	Sources          []string          `json:"sources"`
}
type PackageRequest struct {
	Client, Version, SkillName, SkillSource, ArtifactDigest, ProfileDigest, Endpoint, ProfileID string
	Revision                                                                                    int64
	RequiredControls                                                                            []string
}
type NativeExporter func(PackageRequest) (Package, error)
type PackageOptions struct {
	Endpoint, ProfileID string
	Revision            int64
	RequiredControls    []string
	Exporter            NativeExporter
}

func NativePackage(client, version, name, source, profileDigest string, options ...PackageOptions) (Package, error) {
	if ValidateSource(name, source) != nil || !digestPattern.MatchString(profileDigest) || len(options) != 1 || options[0].Exporter == nil {
		return Package{}, ErrInvalid
	}
	o := options[0]
	if o.RequiredControls == nil {
		o.RequiredControls = []string{"credential_custody", "current_authority", "client_bound_run", "commit_bound_target", "result_reauthorization", "expiry", "revocation", "bounded_output", "explicit_cancellation"}
	}
	p, e := o.Exporter(PackageRequest{Client: client, Version: version, SkillName: name, SkillSource: source, ArtifactDigest: Digest([]byte(source)), ProfileDigest: profileDigest, Endpoint: o.Endpoint, ProfileID: o.ProfileID, Revision: o.Revision, RequiredControls: o.RequiredControls})
	if e != nil || p.Harness != client || p.Version != version || p.ProfileDigest != profileDigest || p.Manifest.ArtifactDigest != Digest([]byte(source)) || p.Manifest.HarnessVersion != version {
		return Package{}, ErrInvalid
	}
	return p, nil
}
