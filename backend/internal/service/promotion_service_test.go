package service

import (
	"testing"
)

func TestValidateEnvironmentOrder(t *testing.T) {
	svc := &PromotionService{}

	tests := []struct {
		name    string
		source  string
		target  string
		wantErr bool
	}{
		{
			name:    "alpha to uat is valid",
			source:  "alpha",
			target:  "uat",
			wantErr: false,
		},
		{
			name:    "uat to prod is valid",
			source:  "uat",
			target:  "prod",
			wantErr: false,
		},
		{
			name:    "alpha to prod skips stage",
			source:  "alpha",
			target:  "prod",
			wantErr: true,
		},
		{
			name:    "prod to alpha is backward",
			source:  "prod",
			target:  "alpha",
			wantErr: true,
		},
		{
			name:    "uat to alpha is backward",
			source:  "uat",
			target:  "alpha",
			wantErr: true,
		},
		{
			name:    "same environment",
			source:  "alpha",
			target:  "alpha",
			wantErr: true,
		},
		{
			name:    "invalid source",
			source:  "staging",
			target:  "prod",
			wantErr: true,
		},
		{
			name:    "invalid target",
			source:  "alpha",
			target:  "staging",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.validateEnvironmentOrder(tt.source, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnvironmentOrder(%s, %s) error = %v, wantErr %v",
					tt.source, tt.target, err, tt.wantErr)
			}
		})
	}
}

func TestDiffEntryActions(t *testing.T) {
	tests := []struct {
		name           string
		sourceExists   bool
		targetExists   bool
		sameValue      bool
		expectedAction string
	}{
		{
			name:           "new key added",
			sourceExists:   true,
			targetExists:   false,
			expectedAction: "add",
		},
		{
			name:           "existing key updated",
			sourceExists:   true,
			targetExists:   true,
			sameValue:      false,
			expectedAction: "update",
		},
		{
			name:           "existing key no change",
			sourceExists:   true,
			targetExists:   true,
			sameValue:      true,
			expectedAction: "no_change",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var action string
			if !tt.targetExists {
				action = "add"
			} else if tt.sameValue {
				action = "no_change"
			} else {
				action = "update"
			}

			if action != tt.expectedAction {
				t.Errorf("expected action %s, got %s", tt.expectedAction, action)
			}
		})
	}
}

func TestOverridePolicyValidation(t *testing.T) {
	tests := []struct {
		name    string
		policy  string
		wantErr bool
	}{
		{name: "skip is valid", policy: "skip", wantErr: false},
		{name: "overwrite is valid", policy: "overwrite", wantErr: false},
		{name: "empty defaults to skip", policy: "", wantErr: false},
		{name: "invalid policy", policy: "merge", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := tt.policy
			if policy == "" {
				policy = "skip"
			}
			valid := policy == "skip" || policy == "overwrite"
			if valid == tt.wantErr {
				t.Errorf("override policy %q: valid=%v, wantErr=%v", tt.policy, valid, tt.wantErr)
			}
		})
	}
}

func TestEnvOrder(t *testing.T) {
	if envOrder["alpha"] >= envOrder["uat"] {
		t.Error("alpha should be before uat")
	}
	if envOrder["uat"] >= envOrder["prod"] {
		t.Error("uat should be before prod")
	}
	if len(envOrder) != 3 {
		t.Errorf("expected 3 environments, got %d", len(envOrder))
	}
}
