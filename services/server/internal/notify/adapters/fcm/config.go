// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"errors"
	"time"
)

// The configuration keys of the adapter (13 17); error texts name them, never a value.
const (
	// KeyProjectID is push.fcm.project_id, the Firebase project of the send URL.
	KeyProjectID = "push.fcm.project_id"
	// KeyServiceAccountRef is push.fcm.service_account_ref, the reference of the service account
	// JSON in the secret provider.
	KeyServiceAccountRef = "push.fcm.service_account_ref"
)

// Config holds the keys push.fcm.*, push.ttl and push.retry_cap (13 17); internal/wire binds it
// from the loaded configuration.
type Config struct {
	// ProjectID is push.fcm.project_id; ServiceAccountRef is push.fcm.service_account_ref.
	ProjectID, ServiceAccountRef string
	// TTL is push.ttl, written as android.ttl in whole seconds; PauseFor is push.retry_cap, the
	// length of the pause after a credential error (Design note 4 c of brief M3-21).
	TTL, PauseFor time.Duration
}

// Enabled reports whether FCM is configured: exactly when push.fcm.project_id is set. Without it
// internal/wire builds no adapter and reads no secret.
func (c Config) Enabled() bool { return c.ProjectID != "" }

// errProjectID is the error of the cross-check push_fcm_project_id; it names the key only.
var errProjectID = errors.New(KeyProjectID + " must consist of lower-case letters, digits and hyphens and start with a letter")

// Validate is the cross-check push_fcm_project_id (02 13 item 4): a non-empty push.fcm.project_id
// holds only lower-case letters, digits and hyphens and starts with a letter, because the value
// becomes a path segment of the send URL. An empty project ID (FCM disabled) is valid. The error
// names the key, never the value.
//
// Example:
//
//	err := fcm.Config{ProjectID: "example-chat"}.Validate() // nil
func (c Config) Validate() error {
	return ValidateProjectID(c.ProjectID)
}

// ValidateProjectID applies the rule of Validate to one value of push.fcm.project_id; the
// configuration cross-check of internal/wire calls it. The error names the key, never id.
func ValidateProjectID(id string) error {
	if id == "" {
		return nil
	}
	if id[0] < 'a' || id[0] > 'z' {
		return errProjectID
	}
	for i := range len(id) {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return errProjectID
		}
	}
	return nil
}
