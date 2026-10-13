// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// SecretResolver resolves a secret reference such as "file:<secrets.dir>/s3_access_key";
// *secrets.Provider implements it. The provider caches a value for secrets.cache_ttl and keeps
// the old value when a re-read fails (docs/spec/16-betrieb.md "5.2").
type SecretResolver interface {
	Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error)
}

// The keys the errors of the credentials name.
const (
	keyAccessRef = "file.s3.access_key_ref"
	keySecretRef = "file.s3.secret_key_ref" // #nosec G101 -- a configuration key name, not a credential
)

// credentialError is a failed resolution of one of the two references. Its text names the key and
// the error code of the secret provider, never a value or the reference.
type credentialError struct {
	key string
	err error
}

func (e *credentialError) Error() string {
	return e.key + ": " + secrets.ErrorCode(e.err)
}

func (e *credentialError) Unwrap() error { return e.err }

// secretCredentials is the credentials provider of the SDK client (docs/DECISIONS.md M3-02/6). It
// resolves both references through the secret provider on every Retrieve and marks the result
// as expired at once, so the credential cache of the SDK asks again for the next request and the
// driver keeps no copy of its own; a key rotated on disk is used once the secret provider reads it
// again after secrets.cache_ttl, without restart (02 "13" item 6).
type secretCredentials struct {
	resolver             SecretResolver
	accessRef, secretRef string
	now                  func() time.Time
}

// Retrieve implements aws.CredentialsProvider. Errors name the key, never a value.
func (c secretCredentials) Retrieve(ctx context.Context) (aws.Credentials, error) {
	access, _, err := c.resolver.Resolve(ctx, c.accessRef)
	if err != nil {
		return aws.Credentials{}, &credentialError{key: keyAccessRef, err: err}
	}
	secret, _, err := c.resolver.Resolve(ctx, c.secretRef)
	if err != nil {
		return aws.Credentials{}, &credentialError{key: keySecretRef, err: err}
	}
	return aws.Credentials{
		AccessKeyID:     string(access.Bytes()),
		SecretAccessKey: string(secret.Bytes()),
		Source:          "server secret provider",
		CanExpire:       true,
		Expires:         c.now(),
	}, nil
}

// check resolves both references once, so a missing or insecure key file stops the start
// (02 "14.4" row Secret-Provider). The error names the key and wraps the error of the provider.
func (c secretCredentials) check(ctx context.Context) error {
	for _, ref := range []struct{ key, raw string }{{keyAccessRef, c.accessRef}, {keySecretRef, c.secretRef}} {
		if _, _, err := c.resolver.Resolve(ctx, ref.raw); err != nil {
			return fmt.Errorf("%s: %w", ref.key, err)
		}
	}
	return nil
}

var _ aws.CredentialsProvider = secretCredentials{}
