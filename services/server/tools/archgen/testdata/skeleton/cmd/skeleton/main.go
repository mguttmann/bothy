// SPDX-License-Identifier: Apache-2.0

// Package main is the binary of the fixture.
package main

import (
	"example.com/skeleton/internal/wire"
	"example.com/skeleton/migrations"
)

func main() {
	_ = wire.Build()
	_ = migrations.Latest
}
