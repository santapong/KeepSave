package vault

import (
	"context"
	"errors"

	"github.com/santapong/KeepSave/backend/internal/crypto"
)

// PayloadCustody keeps short-lived mail proofs and operation results out of
// recovery bundles. Callers receive plaintext only inside a bounded callback.
type PayloadCustody struct{ crypto *crypto.Service }

func NewPayloadCustody(c *crypto.Service) *PayloadCustody { return &PayloadCustody{c} }
func (c *PayloadCustody) Seal(ctx context.Context, plain []byte) ([]byte, []byte, error) {
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	if c == nil || c.crypto == nil || len(plain) > 4<<20 {
		return nil, nil, errors.New("payload custody unavailable")
	}
	return c.crypto.EncryptServiceSecret(plain)
}
func (c *PayloadCustody) WithOpened(ctx context.Context, cipher, nonce []byte, fn func([]byte) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if c == nil || c.crypto == nil || len(cipher) > (4<<20)+32 || fn == nil {
		return errors.New("payload custody unavailable")
	}
	plain, err := c.crypto.DecryptServiceSecret(cipher, nonce)
	if err != nil {
		return errors.New("payload authentication failed")
	}
	defer crypto.SecureZero(plain)
	return fn(plain)
}
