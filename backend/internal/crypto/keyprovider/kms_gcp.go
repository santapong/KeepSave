package keyprovider

import (
	"context"
	"encoding/base64"
	"fmt"
)

// GCPKMSDecrypter is the narrow interface GCPKMSProvider needs. Real impl
// is a thin adapter over cloud.google.com/go/kms/apiv1.KeyManagementClient.
type GCPKMSDecrypter interface {
	Decrypt(ctx context.Context, keyName string, ciphertext []byte) ([]byte, error)
}

// GCPKMSProvider decrypts a wrapped data key from Google Cloud KMS.
type GCPKMSProvider struct {
	dec        GCPKMSDecrypter
	keyName    string
	ciphertext []byte
}

// NewGCPKMSProvider builds the provider. keyName is the fully-qualified
// projects/*/locations/*/keyRings/*/cryptoKeys/* resource name.
func NewGCPKMSProvider(dec GCPKMSDecrypter, keyName, ciphertextB64 string) (*GCPKMSProvider, error) {
	if dec == nil {
		return nil, fmt.Errorf("nil GCPKMSDecrypter")
	}
	if keyName == "" {
		return nil, fmt.Errorf("empty GCP KMS keyName")
	}
	ct, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("decoding KMS ciphertext: %w", err)
	}
	return &GCPKMSProvider{dec: dec, keyName: keyName, ciphertext: ct}, nil
}

func (p *GCPKMSProvider) Name() string { return "gcpkms" }

func (p *GCPKMSProvider) GetMasterKey(ctx context.Context) ([]byte, error) {
	key, err := p.dec.Decrypt(ctx, p.keyName, p.ciphertext)
	if err != nil {
		return nil, fmt.Errorf("gcp kms decrypt: %w", err)
	}
	if err := validateKey(key); err != nil {
		return nil, err
	}
	return key, nil
}

func (p *GCPKMSProvider) Rotate(_ context.Context) error { return ErrUnsupported }
