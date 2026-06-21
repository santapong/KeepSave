package config

import "testing"

func TestParseCommaList(t *testing.T) {
	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"a@x.com", []string{"a@x.com"}},
		{"A@X.com, b@y.com ,", []string{"a@x.com", "b@y.com"}},
		{"a@x.com,a@x.com", []string{"a@x.com"}}, // de-duplicated
	}
	for _, tc := range cases {
		if got := parseCommaList(tc.in); !eq(got, tc.want) {
			t.Errorf("parseCommaList(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
