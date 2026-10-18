// SPDX-License-Identifier: Apache-2.0

//go:build contract_validate

package httpserver

import (
	"net/http"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/contractcheck"
)

// The test build (tag contract_validate) validates every response of the public listener against
// docs/api/openapi.yaml (G8, docs/spec/06-api.md "Contract tests and gate"); the production build
// has no such file and no reference to contractcheck.
func init() { responseCheck, keepResponses = installContractCheck, true }

// installContractCheck wraps h with G8 over the request validator of c: the same parsed contract
// and the operations mounted on c, so the check costs no second build of the contract. A chain
// built over another contract (test fixtures) stays unchecked.
func installContractCheck(c *Chain, h http.Handler) http.Handler {
	v := c.validator
	routes := make([]contractcheck.Route, 0, len(v.ops))
	for id, e := range v.ops {
		for method, op := range e.item.GetOperations().FromOldest() {
			if op.OperationId == id {
				routes = append(routes, contractcheck.Route{ID: id, Method: strings.ToUpper(method), Path: e.path, Item: e.item})
			}
		}
	}
	return contractcheck.Attach(v.specSum, v.base, v.v, routes)(h)
}
