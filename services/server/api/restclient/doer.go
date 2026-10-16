// SPDX-License-Identifier: Apache-2.0

// Package restclient is the API client generated from docs/api/openapi.yaml with every operation
// (api/restclient/gen, docs/DECISIONS.md D-M8-10-1); this file holds the only hand-written part.
package restclient

import "net/http"

// DoerFunc is an HttpRequestDoer from a function, so a user of the client can pass its own
// transport (for example the DPoP transport of the import tool) as a function value of this
// package: the deep scan of go-arch-lint counts a value of a foreign type handed into the client
// as a dependency of api on that component (docs/DECISIONS.md M1-34/12, D-M8-10-8).
//
// Example:
//
//	c, err := restclient.NewClientWithResponses(base, restclient.WithHTTPClient(restclient.DoerFunc(t.RoundTrip)))
type DoerFunc func(req *http.Request) (*http.Response, error)

// Do calls f with req.
func (f DoerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }
