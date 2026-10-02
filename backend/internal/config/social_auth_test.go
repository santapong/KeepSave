package config

import "testing"

func TestSocialAuthConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, origin, env, id, secret string
		valid                         bool
	}{
		{"disabled", "", "development", "", "", true},
		{"local", "http://127.0.0.1:4651", "development", "id", "secret", true},
		{"production", "https://keepsave.example", "production", "id", "secret", true},
		{"partial", "https://keepsave.example", "production", "id", "", false},
		{"missing origin", "", "development", "id", "secret", false},
		{"insecure production", "http://localhost:4651", "production", "id", "secret", false},
		{"insecure remote", "http://keepsave.example", "development", "id", "secret", false},
		{"path", "https://keepsave.example/login", "production", "id", "secret", false},
		{"userinfo", "https://user@keepsave.example", "production", "id", "secret", false},
		{"query", "https://keepsave.example?redirect=evil", "production", "id", "secret", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SOCIAL_AUTH_ORIGIN", tc.origin)
			t.Setenv("GITHUB_CLIENT_ID", tc.id)
			t.Setenv("GITHUB_CLIENT_SECRET", tc.secret)
			t.Setenv("GOOGLE_CLIENT_ID", "")
			t.Setenv("GOOGLE_CLIENT_SECRET", "")
			_, err := loadSocialAuth(tc.env)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t error=%v", tc.valid, err)
			}
		})
	}
}
