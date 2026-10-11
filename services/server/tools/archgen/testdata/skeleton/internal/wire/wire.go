// SPDX-License-Identifier: Apache-2.0

// Package wire assembles the modules of the fixture.
package wire

import (
	"example.com/skeleton/internal/chat/adapters/postgres"
	"example.com/skeleton/internal/chat/app"
	"example.com/skeleton/internal/platform/pg"
)

// Build wires the chat module.
func Build() *postgres.Repository {
	return &postgres.Repository{Pool: &pg.Pool{}, Service: &app.Service{}}
}
