package harness

import (
	"strings"
	"testing"
)

func request(name string) ExportRequest {
	source := "---\nname: keepsave-review-v1\ndescription: Review an approved run.\n---\nUse available_runs and explicit cancel_run.\n"
	return ExportRequest{Harness: name, Endpoint: "https://app.keepsave.example/mcp", ProfileID: "reviewed-profile", ProfileRevision: 1, ProfileDigest: strings.Repeat("a", 64), RequiredControls: []string{"credential_custody", "current_authority", "client_bound_run", "commit_bound_target", "result_reauthorization", "expiry", "revocation", "bounded_output", "explicit_cancellation"}, SkillName: "keepsave-review-v1", SkillSource: source, ArtifactDigest: digest([]byte(source))}
}
func TestExactNativePackagesAndUnsupportedControls(t *testing.T) {
	for _, name := range []string{"codex", "hermes"} {
		r := request(name)
		p, e := Export(r)
		if e != nil {
			t.Fatal(e)
		}
		if ok, issues := Check(p, p.Manifest.HarnessVersion); !ok {
			t.Fatal(issues)
		}
		if p.Manifest.Qualification != "real_harness_acceptance_pending" {
			t.Fatal("source claimed qualification")
		}
		if name == "hermes" {
			if !strings.Contains(string(p.Files["config.yaml"]), "protocol: legacy") || !strings.Contains(string(p.Files["config.yaml"]), "redirect_port: 17702") {
				t.Fatal("Hermes callback/protocol mismatch")
			}
			if ok, _ := Check(p, "0.21.3"); ok {
				t.Fatal("installed old version accepted")
			}
			if string(p.Files["skills/keepsave-review-v1/SKILL.md"]) != r.SkillSource {
				t.Fatal("private source changed")
			}
		} else {
			if strings.Contains(string(p.Files["config.toml"]), "oauth_resource") || !strings.Contains(string(p.Files["config.toml"]), "callback_port = 17701") {
				t.Fatal("Codex export mismatch")
			}
		}
		p.Files["scripts/run.sh"] = []byte("unexpected")
		if ok, _ := Check(p, p.Manifest.HarnessVersion); ok {
			t.Fatal("unexpected executable accepted")
		}
		r.RequiredControls = append(r.RequiredControls, "native_stop")
		if _, e = Export(r); e == nil {
			t.Fatal("unverified required control accepted")
		}
	}
}
func TestArtifactDigestAndPackageTamper(t *testing.T) {
	r := request("hermes")
	r.SkillSource += "changed"
	if _, e := Export(r); e == nil {
		t.Fatal("source digest mismatch accepted")
	}
	r = request("hermes")
	p, e := Export(r)
	if e != nil {
		t.Fatal(e)
	}
	p.Files["config.yaml"] = []byte("modified callback")
	if ok, _ := Check(p, "0.21.5"); ok {
		t.Fatal("modified config accepted")
	}
}
func TestPortableSourceRejectsDuplicateOrExtendedAuthority(t *testing.T) {
	for _, source := range []string{"---\nname: keepsave-review-v1\nname: other\ndescription: test\n---\nbody\n", "---\nname: keepsave-review-v1\ndescription: test\nallowed-tools: shell\n---\nbody\n", "---\nname: keepsave-review-v1\ndescription: test\n---\n"} {
		if ValidateSkill("keepsave-review-v1", source) == nil {
			t.Fatal("unsupported portable source accepted")
		}
	}
}
