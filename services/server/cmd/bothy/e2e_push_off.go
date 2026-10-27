// SPDX-License-Identifier: Apache-2.0

//go:build !e2e

package main

import "github.com/OWNER/bothy/services/server/internal/wire"

// applyE2EPush does nothing in the release build: no environment variable redirects push there
// (brief M5-11 Design note 1). The build with the tag e2e replaces it with e2e_push.go; a test of
// cmd/bothy/internal/e2epush holds that only that build links the endpoints.
func applyE2EPush([]string, *wire.Deps) error { return nil }
