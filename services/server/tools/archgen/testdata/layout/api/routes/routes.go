// SPDX-License-Identifier: Apache-2.0

// Package routes stands in for the generated route table; it calls the constructors of rest.
package routes

import "example.com/skeleton/api/rest"

// Table maps an operation to its handler.
type Table struct {
	Server rest.Server
}
