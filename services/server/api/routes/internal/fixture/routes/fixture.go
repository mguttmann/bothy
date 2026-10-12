// SPDX-License-Identifier: Apache-2.0

// Package routes is the route table of the route generator's fixture spec
// (api/routes/gen/testdata/fixture): the same generated files as the real surface, bound to the
// same hand-written core, so the tests of api/routes run the generated glue of a non-empty
// enable list. Only tests use it.
package routes

import routes "github.com/OWNER/bothy/services/server/api/routes"

// table binds the generated files of this package to the core.
var table = routes.NewTable[Server](operations, catalogEntries, register)

// Table returns the route table of the fixture spec.
func Table() *routes.Table[Server] { return table }
