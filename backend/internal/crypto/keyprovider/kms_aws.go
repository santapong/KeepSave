package keyprovider

import (
	"context"
	"encoding/base64"
	"fmt"
)

// AWSKMSDecrypter is the narrow interface AWSKMSProvider needs. The real
// implementation is *kms.Client from github.com/aws/aws-sdk-go-v2/service/kms;
// wiring is a one-line adapter in main.go, keeping this package SDK-free.
type AWSKMSDecrypter interface {
	Decrypt(ctx context.Context, keyID string, ciphertext []byte) ([]byte, error)
}

// AWSKMSProvider decrypts a pre-wrapped data key blob that the operator
// produced out-of-band (e.g. `aws kms encrypt --key-id alias/keepsave`).
type AWSKMSProvider struct {
	dec        AWSKMSDecrypter
	keyID      string
	ciphertext []byte
}

// NewAWSKMSProvider builds the provider. ciphertextB64 is the base64 blob
// stored in the KEEPSAVE_KMS_CIPHERTEXT env var; keyID is the AWS KMS
// alias or ARN.
func NewAWSKMSProvider(dec AWSKMSDecrypter, keyID, ciphertextB64 string) (*AWSKMSProvider, error) {
	if dec == nil {
		return nil, fmt.Errorf("nil AWSKMSDecrypter")
	}
	if keyID == "" {
		return nil, fmt.Errorf("empty AWS KMS keyID")
	}
	ct, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("decoding KMS ciphertext: %w", err)
	}
	return &AWSKMSProvider{dec: dec, keyID: keyID, ciphertext: ct}, nil
}

func (p *AWSKMSProvider) Name() string { return "awskms" }

func (p *AWSKMSProvider) GetMasterKey(ctx context.Context) ([]byte, error) {
	key, err := p.dec.Decrypt(ctx, p.keyID, p.ciphertext)
	if err != nil {
		return nil, fmt.Errorf("aws kms decrypt: %w", err)
	}
	if err := validateKey(key); err != nil {
		return nil, err
	}
	return key, nil
}

// Rotate is surfaced here so upstream tooling can drive rotation; the
// operator is expected to regenerate the ciphertext blob out-of-band and
// restart the service. Returning ErrUnsupported keeps the interface clean.
func (p *AWSKMSProvider) Rotate(_ context.Context) error { return ErrUnsupported }
