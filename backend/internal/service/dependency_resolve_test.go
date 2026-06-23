package service

import "testing"

func TestResolveEnvReferences(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]string
		key  string
		want string
	}{
		{"no refs", map[string]string{"A": "plain"}, "A", "plain"},
		{"simple ${}", map[string]string{"A": "${B}", "B": "v"}, "A", "v"},
		{"embedded", map[string]string{"URL": "pre${A}post", "A": "X"}, "URL", "preXpost"},
		{"transitive", map[string]string{"A": "${B}", "B": "${C}", "C": "deep"}, "A", "deep"},
		{"dollar form", map[string]string{"A": "$B", "B": "v"}, "A", "v"},
		{"brace form", map[string]string{"A": "{{B}}", "B": "v"}, "A", "v"},
		{"percent form", map[string]string{"A": "%B%", "B": "v"}, "A", "v"},
		{"unknown ref left literal", map[string]string{"A": "${MISSING}"}, "A", "${MISSING}"},
		{"self cycle left literal", map[string]string{"A": "${A}"}, "A", "${A}"},
		{"compose", map[string]string{
			"DB_USER": "admin", "DB_PASS": "s3cret",
			"DATABASE_URL": "postgres://${DB_USER}:${DB_PASS}@host/db",
		}, "DATABASE_URL", "postgres://admin:s3cret@host/db"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveEnvReferences(tc.raw)
			if got[tc.key] != tc.want {
				t.Errorf("%s = %q, want %q", tc.key, got[tc.key], tc.want)
			}
		})
	}
}

// TestResolveEnvReferences_MutualCycleTerminates ensures A=${B}, B=${A} does not
// loop and both keys come back (with the cycle edge left literal).
func TestResolveEnvReferences_MutualCycleTerminates(t *testing.T) {
	got := ResolveEnvReferences(map[string]string{"A": "${B}", "B": "${A}"})
	if _, ok := got["A"]; !ok {
		t.Fatal("A missing from result")
	}
	if _, ok := got["B"]; !ok {
		t.Fatal("B missing from result")
	}
	// The exact literal depends on which edge breaks; the contract is only that
	// it terminates and returns both keys, which reaching here proves.
}

// TestResolveEnvReferences_DepthCap ensures a long chain terminates at the cap.
func TestResolveEnvReferences_DepthCap(t *testing.T) {
	raw := map[string]string{}
	// K0 -> K1 -> ... -> K40 = "end"; chain longer than maxResolveDepth.
	for i := 0; i < 40; i++ {
		raw["K"+itoa(i)] = "${K" + itoa(i+1) + "}"
	}
	raw["K40"] = "end"
	got := ResolveEnvReferences(raw)
	// Must terminate; deep-enough keys resolve, the cap leaves the rest literal.
	if got["K39"] != "end" && got["K39"] == "" {
		t.Errorf("unexpected empty resolution at K39")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
