package config

import "testing"

func TestPoolConfiguration(t *testing.T) {
	for _, tc := range []struct {
		open, idle string
		valid      bool
	}{
		{"4", "1", true}, {"1", "0", true}, {"0", "0", false},
		{"4", "5", false}, {"4", "-1", false}, {"oops", "1", false},
	} {
		t.Run(tc.open+"_"+tc.idle, func(t *testing.T) {
			setenv(t, map[string]string{"DATABASE_URL": "sqlite://test.db", "MASTER_KEY": goodKey(), "JWT_SECRET": "test", "DB_MAX_OPEN_CONNS": tc.open, "DB_MAX_IDLE_CONNS": tc.idle})
			cfg, err := Load()
			if (err == nil) != tc.valid {
				t.Fatalf("expected valid=%v, got err=%v", tc.valid, err)
			}
			if tc.valid && cfg.DBMaxIdleConns > cfg.DBMaxOpenConns {
				t.Fatal("idle exceeds open")
			}
		})
	}
}
