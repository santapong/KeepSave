package service

import (
	"testing"
)

func TestFindReferences(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected []string
	}{
		{
			"dollar-brace syntax",
			"postgres://${DB_USER}:${DB_PASS}@localhost/db",
			[]string{"DB_USER", "DB_PASS"},
		},
		{
			"dollar-only syntax",
			"redis://$REDIS_HOST:$REDIS_PORT",
			[]string{"REDIS_HOST", "REDIS_PORT"},
		},
		{
			"mustache syntax",
			"https://{{API_HOST}}/v1",
			[]string{"API_HOST"},
		},
		{
			"percent syntax",
			"Server=%DB_SERVER%;Port=%DB_PORT%",
			[]string{"DB_SERVER", "DB_PORT"},
		},
		{
			"no references",
			"plain-value-123",
			nil,
		},
		{
			"mixed syntax",
			"${HOST}:$PORT/{{PATH}}",
			[]string{"HOST", "PORT", "PATH"},
		},
		{
			"duplicate references",
			"${VAR}=${VAR}",
			[]string{"VAR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := findReferences(tt.value)
			var keys []string
			for _, r := range refs {
				keys = append(keys, r.key)
			}

			if tt.expected == nil && keys != nil {
				t.Errorf("expected nil refs, got %v", keys)
				return
			}

			if len(keys) != len(tt.expected) {
				t.Errorf("got %d refs, want %d: %v", len(keys), len(tt.expected), keys)
				return
			}

			for i, key := range keys {
				if key != tt.expected[i] {
					t.Errorf("ref[%d] = %q, want %q", i, key, tt.expected[i])
				}
			}
		})
	}
}

func TestGetBuiltinTemplates(t *testing.T) {
	templates := GetBuiltinTemplates()

	if len(templates) == 0 {
		t.Fatal("expected at least one builtin template")
	}

	stacks := make(map[string]bool)
	for _, tmpl := range templates {
		if tmpl.Name == "" {
			t.Error("template name should not be empty")
		}
		if tmpl.Stack == "" {
			t.Error("template stack should not be empty")
		}
		if !tmpl.IsGlobal {
			t.Errorf("builtin template %q should be global", tmpl.Name)
		}
		stacks[tmpl.Stack] = true

		keysData, ok := tmpl.Keys["keys"]
		if !ok {
			t.Errorf("template %q missing keys field", tmpl.Name)
			continue
		}
		keysList, ok := keysData.([]map[string]interface{})
		if !ok {
			t.Errorf("template %q keys has unexpected type", tmpl.Name)
			continue
		}
		if len(keysList) == 0 {
			t.Errorf("template %q should have at least one key", tmpl.Name)
		}
	}

	expectedStacks := []string{"nodejs", "python", "go", "aws"}
	for _, s := range expectedStacks {
		if !stacks[s] {
			t.Errorf("expected builtin template for stack %q", s)
		}
	}
}
