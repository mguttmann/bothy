// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-tls-verify (run by `make semgrep`).
package fixtures

import (
	"crypto/tls"

	ctls "crypto/tls"
)

func insecure(cfg *tls.Config, skip bool) {
	// ruleid: bothy-go-tls-verify
	_ = &tls.Config{InsecureSkipVerify: true}
	// ruleid: bothy-go-tls-verify
	_ = tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: skip}
	// ruleid: bothy-go-tls-verify
	_ = &tls.Config{
		ServerName:         "api.example.com",
		InsecureSkipVerify: !false,
	}
	// ruleid: bothy-go-tls-verify
	cfg.InsecureSkipVerify = true
	// ruleid: bothy-go-tls-verify
	cfg.InsecureSkipVerify = skip
	// ruleid: bothy-go-tls-verify
	_ = &ctls.Config{InsecureSkipVerify: true}
}

func verified(cfg *tls.Config) {
	// ok: bothy-go-tls-verify
	_ = &tls.Config{InsecureSkipVerify: false}
	// ok: bothy-go-tls-verify
	_ = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: nil}
	// ok: bothy-go-tls-verify
	cfg.InsecureSkipVerify = false
	// ok: bothy-go-tls-verify
	_ = cfg.InsecureSkipVerify
}
