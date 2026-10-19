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
func init() { responseCheck = installContractCheck }

// installContractCheck wraps h with G8 over the request validator of c: the same parsed contract
// and the operations mounted on c, so the check costs no second build of the contract. A chain
// built over another contract (test fixtures) stays unchecked. A request that a route with
// AuthorizedInBand serves (the gateway, the control connection of the host services) is no
// operation of the contract and passes unchecked, also where a contract pattern matches its path
// too (GET /api/v1/hosts/connect and getHost, INT-W45/3).
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
	checked := contractcheck.Attach(v.specSum, v.base, v.v, routes)(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if servesInBand(c, r) {
			h.ServeHTTP(w, r)
			return
		}
		checked.ServeHTTP(w, r)
	})
}

// servesInBand reports whether a route with AuthorizedInBand of c matches r.
func servesInBand(c *Chain, r *http.Request) bool {
	pats := c.inBandPatterns()
	if len(pats) == 0 {
		return false
	}
	mux := http.NewServeMux()
	for _, p := range pats {
		mux.Handle(p, http.NotFoundHandler())
	}
	_, pattern := mux.Handler(r)
	return pattern != ""
}
