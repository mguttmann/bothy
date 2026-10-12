// SPDX-License-Identifier: Apache-2.0

// Package rest is the REST layer generated from the contract by api/routes/gen (oapi-codegen:
// models, std-http and strict server of the enabled operations) and the embedded contract copy.
package rest

import _ "embed" // the contract copy

// Spec is the fixture contract byte for byte (openapi.gen.yaml, written by api/routes/gen like
// the real api/rest.Spec), so tests validate fixture requests with the HTTP chain.
//
//go:embed openapi.gen.yaml
var Spec []byte
