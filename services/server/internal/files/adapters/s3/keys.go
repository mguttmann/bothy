// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// objectName returns the S3 object name of key: "<prefix>/<key>" with file.s3.prefix set, else
// the key (docs/spec/12-files.md "4.4", docs/DECISIONS.md M3-02/7). Key and prefix both follow
// the one key grammar of all drivers (domain.ValidateKey), which refuses empty segments and a
// leading or trailing "/", so no two keys collapse into one object in a path-style URL and the
// name is again in the grammar. The driver adds nothing else to a key (SEC-UP-8).
//
// The error wraps domain.ErrInvalidKey (app.ErrInvalidKey) and never repeats key or prefix.
func objectName(prefix, key string) (string, error) {
	if err := domain.ValidateKey(key); err != nil {
		return "", err
	}
	if prefix == "" {
		return key, nil
	}
	if err := domain.ValidateKey(prefix); err != nil {
		return "", fmt.Errorf("file.s3.prefix: %w", err)
	}
	return prefix + "/" + key, nil
}
