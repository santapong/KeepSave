// Package harness renders versioned native packages from a portable profile.
// Packages contain instructions and public configuration, never credentials.
package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/santapong/KeepSave/backend/internal/mcpgateway/catalog"
	"gopkg.in/yaml.v3"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const PackageVersion = "keepsave-native-v1"

type ExportRequest struct {
	Harness          string   `json:"harness"`
	Endpoint         string   `json:"endpoint"`
	ProfileID        string   `json:"profile_id"`
	ProfileRevision  int64    `json:"profile_revision"`
	ProfileDigest    string   `json:"profile_digest"`
	RequiredControls []string `json:"required_controls"`
	SkillName        string   `json:"skill_name"`
	SkillSource      string   `json:"skill_source"`
	ArtifactDigest   string   `json:"artifact_digest"`
}
type Manifest struct {
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
type Package struct {
	Files    map[string][]byte
	Manifest Manifest
}

var ErrIncompatible = errors.New("required harness capability unavailable")
var skillName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

func ValidateSkill(name, source string) error {
	if !skillName.MatchString(name) || len(source) < 1 || len(source) > 64<<10 || strings.ContainsRune(source, 0) || !strings.HasPrefix(source, "---\n") {
		return ErrIncompatible
	}
	parts := strings.SplitN(strings.TrimPrefix(source, "---\n"), "\n---\n", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return ErrIncompatible
	}
	// First release deliberately accepts the portable name/description subset.
	// Unknown extensions, duplicate keys and executable dependencies require a
	// new reviewed format, never an implicit restriction downgrade.
	var front struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	decoder := yaml.NewDecoder(strings.NewReader(parts[0]))
	decoder.KnownFields(true)
	if e := decoder.Decode(&front); e != nil || front.Name != name || strings.TrimSpace(front.Description) == "" || len(front.Description) > 1024 {
		return ErrIncompatible
	}
	return nil
}
func digest(b []byte) string { d := sha256.Sum256(b); return hex.EncodeToString(d[:]) }
func treeDigest(files map[string]string) string {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		b.WriteString(n)
		b.WriteByte(0)
		b.WriteString(files[n])
		b.WriteByte('\n')
	}
	return digest([]byte(b.String()))
}
func Export(r ExportRequest) (Package, error) {
	u, e := url.Parse(r.Endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "/mcp" || u.RawQuery != "" || u.Fragment != "" || r.ProfileID == "" || r.ProfileRevision < 1 {
		return Package{}, ErrIncompatible
	}
	b, e := hex.DecodeString(r.ProfileDigest)
	if e != nil || len(b) != 32 {
		return Package{}, ErrIncompatible
	}
	if ValidateSkill(r.SkillName, r.SkillSource) != nil || digest([]byte(r.SkillSource)) != r.ArtifactDigest {
		return Package{}, ErrIncompatible
	}
	controls := map[string]string{"credential_custody": "server", "current_authority": "server", "client_bound_run": "server", "commit_bound_target": "server", "result_reauthorization": "server", "expiry": "server", "revocation": "server", "bounded_output": "server", "explicit_cancellation": "server", "native_stop": "unverified", "native_skill_immutability": "unavailable", "skills_over_mcp": "unavailable", "native_tool_filter": "administrator"}
	for _, c := range r.RequiredControls {
		if controls[c] != "server" {
			return Package{}, ErrIncompatible
		}
	}
	m := Manifest{SchemaVersion: "1", PackageVersion: PackageVersion, Harness: r.Harness, Protocol: "2025-11-25", OS: "linux", Endpoint: r.Endpoint, ProfileID: r.ProfileID, ProfileRevision: r.ProfileRevision, ProfileDigest: r.ProfileDigest, SkillName: r.SkillName, ArtifactDigest: r.ArtifactDigest, RequiredControls: append([]string(nil), r.RequiredControls...), Controls: controls, FileDigests: map[string]string{}, Qualification: "real_harness_acceptance_pending", SourceCheckedAt: "2026-10-02"}
	files := map[string][]byte{}
	skillRoot := ""
	switch r.Harness {
	case "codex":
		m.HarnessVersion = "0.153.3"
		m.SDKVersion = "rmcp@3.1.3"
		m.ClientID = "keepsave-codex-linux-v1"
		m.Callback = "http://127.0.0.1:17701/callback"
		skillRoot = ".agents/skills/" + r.SkillName
		files["config.toml"] = []byte(fmt.Sprintf("[mcp_servers.keepsave]\nurl = %q\nscopes = [\"keepsave:tools\", \"offline_access\"]\ntool_timeout_sec = 30\nrequired = true\n\n[mcp_servers.keepsave.oauth]\nclient_id = %q\ncallback_port = 17701\ncallback_url = %q\n", r.Endpoint, m.ClientID, m.Callback))
		m.Sources = []string{"https://learn.chatgpt.com/docs/config-file/config-reference", "https://learn.chatgpt.com/docs/extend/mcp?surface=cli", "https://raw.githubusercontent.com/openai/codex/rust-v0.153.3/codex-rs/rmcp-client/src/perform_oauth_login.rs", "https://raw.githubusercontent.com/modelcontextprotocol/rust-sdk/rmcp-v3.1.3/crates/rmcp/src/transport/auth.rs"}
	case "hermes":
		m.HarnessVersion = "0.21.5"
		m.SDKVersion = "mcp==2.0.0"
		m.ClientID = "keepsave-hermes-linux-v1"
		m.Callback = "http://127.0.0.1:17702/callback"
		skillRoot = "skills/" + r.SkillName
		files["config.yaml"] = []byte(fmt.Sprintf("# Import into a separate Hermes profile; retain the chosen non-Anthropic provider.\nmcp_servers:\n  keepsave:\n    url: %q\n    auth: oauth\n    protocol: legacy\n    strict_redirect_headers: true\n    timeout: 30\n    oauth:\n      client_id: %q\n      token_endpoint_auth_method: none\n      redirect_host: 127.0.0.1\n      redirect_port: 17702\n      scope: keepsave:tools offline_access\n", r.Endpoint, m.ClientID))
		m.Sources = []string{"https://github.com/NousResearch/hermes-agent/releases/tag/v2026.9.24", "https://raw.githubusercontent.com/NousResearch/hermes-agent/v2026.9.24/pyproject.toml", "https://raw.githubusercontent.com/NousResearch/hermes-agent/v2026.9.24/tools/mcp_oauth.py", "https://raw.githubusercontent.com/NousResearch/hermes-agent/v2026.9.24/tools/mcp_tool_transport.py", "https://raw.githubusercontent.com/modelcontextprotocol/python-sdk/v2.0.0/src/mcp/client/auth/oauth2.py"}
	default:
		return Package{}, ErrIncompatible
	}
	files[skillRoot+"/SKILL.md"] = []byte(r.SkillSource)
	profile, e := json.MarshalIndent(map[string]any{"profile_id": r.ProfileID, "profile_revision": r.ProfileRevision, "profile_digest": r.ProfileDigest, "required_controls": r.RequiredControls}, "", "  ")
	if e != nil {
		return Package{}, e
	}
	files[skillRoot+"/references/profile.json"] = append(profile, '\n')
	catalogJSON, e := json.MarshalIndent(catalog.Definitions(), "", "  ")
	if e != nil {
		return Package{}, e
	}
	files[skillRoot+"/references/tools.json"] = append(catalogJSON, '\n')
	files["IMPORT.md"] = []byte("This is an export, not an installation or a proof of compatibility. Review the manifest and create a separate qualification profile. Import the public MCP configuration into that profile; do not overwrite existing configuration or copy provider credentials. Install the instruction tree in the native skill location shown by the package. Read references/tools.json for installation-qualified tool identities and input schema digests; logical operation names in the preserved source refer to that exact catalog. Use keepsave_v1__operation_status for metadata and keepsave_v1__operation_result for current-authority result delivery. Use keepsave_v1__cancel_run to cancel an accepted run; native Stop is separate. Hermes must retain the user's chosen non-Anthropic provider and separate memory/session authority. Native packages remain mutable. Required server controls come from stored KeepSave authority, not the client manifest. Real OAuth consent, native skill loading and client interruption remain acceptance gates.\n")
	for n, v := range files {
		m.FileDigests[n] = digest(v)
	}
	m.TreeDigest = treeDigest(m.FileDigests)
	encoded, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return Package{}, e
	}
	files["manifest.json"] = append(encoded, '\n')
	return Package{Files: files, Manifest: m}, nil
}

// Check never promotes a package to qualified. Metadata and hashes cannot prove
// that a real client imported settings or that server/runner controls were used.
func Check(p Package, actualVersion string) (bool, []string) {
	issues := []string{}
	if actualVersion != p.Manifest.HarnessVersion {
		issues = append(issues, "installed metadata does not match the pinned test candidate")
	}
	prefix := "skills/"
	if p.Manifest.Harness == "codex" {
		prefix = ".agents/skills/"
	}
	source := string(p.Files[prefix+p.Manifest.SkillName+"/SKILL.md"])
	expected, e := Export(ExportRequest{Harness: p.Manifest.Harness, Endpoint: p.Manifest.Endpoint, ProfileID: p.Manifest.ProfileID, ProfileRevision: p.Manifest.ProfileRevision, ProfileDigest: p.Manifest.ProfileDigest, RequiredControls: p.Manifest.RequiredControls, SkillName: p.Manifest.SkillName, SkillSource: source, ArtifactDigest: p.Manifest.ArtifactDigest})
	if e != nil {
		return false, append(issues, "unsupported package or required controls")
	}
	if p.Manifest.TreeDigest != expected.Manifest.TreeDigest || p.Manifest.PackageVersion != PackageVersion || p.Manifest.SDKVersion != expected.Manifest.SDKVersion || p.Manifest.Protocol != expected.Manifest.Protocol || p.Manifest.ClientID != expected.Manifest.ClientID || p.Manifest.Callback != expected.Manifest.Callback {
		issues = append(issues, "manifest differs from the reviewed renderer")
	}
	for n, v := range expected.Files {
		actual, ok := p.Files[n]
		if !ok || string(actual) != string(v) {
			issues = append(issues, "package file mismatch: "+n)
		}
	}
	for n := range p.Files {
		if _, ok := expected.Files[n]; !ok {
			issues = append(issues, "unexpected package file: "+n)
		}
	}
	sort.Strings(issues)
	return len(issues) == 0, issues
}
