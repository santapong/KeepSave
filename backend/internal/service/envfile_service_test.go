package service

import (
	"testing"
)

func TestParseEnvContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			"basic key-value",
			"KEY=value",
			map[string]string{"KEY": "value"},
		},
		{
			"multiple lines",
			"A=1\nB=2\nC=3",
			map[string]string{"A": "1", "B": "2", "C": "3"},
		},
		{
			"comments and empty lines",
			"# This is a comment\n\nKEY=value\n# Another comment",
			map[string]string{"KEY": "value"},
		},
		{
			"double-quoted values",
			`KEY="hello world"`,
			map[string]string{"KEY": "hello world"},
		},
		{
			"single-quoted values",
			`KEY='hello world'`,
			map[string]string{"KEY": "hello world"},
		},
		{
			"export prefix",
			"export KEY=value",
			map[string]string{"KEY": "value"},
		},
		{
			"value with equals sign",
			"URL=postgres://user:pass@host/db?opt=val",
			map[string]string{"URL": "postgres://user:pass@host/db?opt=val"},
		},
		{
			"empty value",
			"KEY=",
			map[string]string{"KEY": ""},
		},
		{
			"empty content",
			"",
			map[string]string{},
		},
		{
			"escaped quotes",
			`KEY="say \"hello\""`,
			map[string]string{"KEY": `say "hello"`},
		},
		{
			"line without equals",
			"INVALID_LINE",
			map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEnvContent(tt.content)

			if len(result) != len(tt.expected) {
				t.Errorf("got %d vars, want %d: %v", len(result), len(tt.expected), result)
				return
			}

			for key, expected := range tt.expected {
				got, ok := result[key]
				if !ok {
					t.Errorf("missing key %q", key)
					continue
				}
				if got != expected {
					t.Errorf("key %q = %q, want %q", key, got, expected)
				}
			}
		})
	}
}
