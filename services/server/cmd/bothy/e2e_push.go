// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package main

import (
	"github.com/OWNER/bothy/services/server/cmd/bothy/internal/e2epush"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// applyE2EPush points the push adapters of the E2E server build at the fake providers named in
// env (brief M5-11 Design note 1; e2epush.Apply). Only `go build -tags e2e` compiles this file:
// the target e2e-server of services/server/Dockerfile, used by the mobile E2E overlay
// infra/compose/test/compose.e2e-mobile.yaml. The error reports a missing or malformed variable.
func applyE2EPush(env []string, d *wire.Deps) error { return e2epush.Apply(env, d) }
