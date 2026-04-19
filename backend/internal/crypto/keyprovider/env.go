package keyprovider

import (
	"context"
	"encoding/base64"
	"fmt"
)

// EnvProvider reads a base64-encoded 32-byte master key from a single env
// var. Intended for local development and single-node docker-compose.
// Production deployments should use AWS/GCP/Vault instead.
type EnvProvider struct {
	varName string
	lookup  func(string) (string, bool)
}

// NewEnvProvider returns an EnvProvider that reads varName at each call.
// Pass lookup = os.LookupEnv in main; tests can inject a fake map.
func NewEnvProvider(varName string, lookup func(string) (string, bool)) *EnvProvider {
	return &EnvProvider{varName: varName, lookup: lookup}
}

func (p *EnvProvider) Name() string { return "env" }

func (p *EnvProvider) GetMasterKey(_ context.Context) ([]byte, error) {
	v, ok := p.lookup(p.varName)
	if !ok || v == "" {
		return nil, fmt.Errorf("%s is not set", p.varName)
	}
	key, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", p.varName, err)
	}
	if err := validateKey(key); err != nil {
		return nil, err
	}
	return key, nil
}

// Rotate is not supported for the env provider. Rotating requires
// regenerating the env var out-of-band and restarting the process.
func (p *EnvProvider) Rotate(_ context.Context) error { return ErrUnsupported }
