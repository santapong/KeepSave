package service

import (
	"errors"
	"testing"
	"time"
)

// TestAPIKeyExpiry_DefaultsAndBounds covers ADR-0009 / audit S-M4: the
// service ALWAYS persists a non-NULL expires_at and refuses values that
// are in the past or beyond the 365-day ceiling.
//
// We test the validation logic in isolation - constructing a real
// APIKeyService requires repos we'd have to stub for unit coverage. The
// branch under test runs before any repo call, so we exercise it via a
// minimal value-only helper.
func TestAPIKeyExpiry_DefaultsAndBounds(t *testing.T) {
	now := time.Now().UTC()

	cases := []struct {
		name      string
		input     *time.Time
		want      time.Duration
		wantError error
	}{
		{
			name:  "default 90 days when nil",
			input: nil,
			want:  apiKeyDefaultTTL,
		},
		{
			name:      "rejected when in past",
			input:     pt(now.Add(-1 * time.Hour)),
			wantError: ErrAPIKeyExpiryOutOfRange,
		},
		{
			name:      "rejected when beyond ceiling",
			input:     pt(now.Add(apiKeyMaxTTL + time.Hour)),
			wantError: ErrAPIKeyExpiryOutOfRange,
		},
		{
			name:  "accepted within ceiling",
			input: pt(now.Add(180 * 24 * time.Hour)),
			want:  180 * 24 * time.Hour,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eff, err := computeEffectiveAPIKeyExpiry(now, tc.input)
			if tc.wantError != nil {
				if !errors.Is(err, tc.wantError) {
					t.Errorf("err = %v, want %v", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			delta := eff.Sub(now)
			tolerance := 2 * time.Second
			if delta < tc.want-tolerance || delta > tc.want+tolerance {
				t.Errorf("effective TTL = %v, want %v +/- %v", delta, tc.want, tolerance)
			}
		})
	}
}

func pt(t time.Time) *time.Time { return &t }
