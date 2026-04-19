// Package keyprovider defines a pluggable master-key source for the
// envelope-encryption layer. The default EnvProvider reads a base64
// MASTER_KEY from the environment (backwards compatible with pre-1.1 dev
// workflows). The AWS / GCP / Vault providers accept narrow client
// interfaces so callers can inject the real SDK client without this
// package importing the SDK itself.
package keyprovider

import (
	"context"
	"errors"
	"fmt"
)

// ErrUnsupported is returned by providers that do not implement an
// optional capability such as Rotate.
var ErrUnsupported = errors.New("operation not supported by this provider")

// Provider is the pluggable source of the 32-byte master key that wraps
// per-project DEKs.
type Provider interface {
	Name() string
	GetMasterKey(ctx context.Context) ([]byte, error)
	Rotate(ctx context.Context) error
}

// validateKey enforces the 32-byte AES-256 key size. All providers share
// this check so callers cannot smuggle a short or long key past startup.
func validateKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("master key must be 32 bytes, got %d", len(key))
	}
	return nil
}
