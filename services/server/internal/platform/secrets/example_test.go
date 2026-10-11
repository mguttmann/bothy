// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"context"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

// internal/wire builds one provider per process; cmd/bothy passes the environment it read once.
// A Value prints as [redacted], and errors name the reference and a code, never a value.
func ExampleProvider_Resolve() {
	p, err := secrets.NewProvider(secrets.Options{
		Dir:      "/run/bothy/secrets",
		CacheTTL: 5 * time.Minute,
		Env:      []string{"BOTHY_SECRET_EXAMPLE=EXAMPLE-value"},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	ctx := context.Background()

	v, _, err := p.Resolve(ctx, "env:BOTHY_SECRET_EXAMPLE")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(v, v.Len())

	_, _, err = p.Resolve(ctx, "env:BOTHY_SECRET_MISSING")
	fmt.Println(secrets.ErrorCode(err), err)

	_, _, err = p.Resolve(ctx, "file:relative/path")
	fmt.Println(secrets.ErrorCode(err), err)
	// Output:
	// [redacted] 13
	// not_found secrets: env:BOTHY_SECRET_MISSING: secret not found: variable is not set
	// invalid_ref secrets: invalid reference: file reference must be an absolute path
}
