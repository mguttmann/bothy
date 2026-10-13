// SPDX-License-Identifier: Apache-2.0

// Package rest is the REST layer generated from the contract by api/routes/gen (oapi-codegen:
// models, std-http and strict server of the enabled operations) and the embedded contract copy.
package rest

import _ "embed" // the contract copy

// Spec is docs/api/openapi.yaml byte for byte (openapi.gen.yaml, written by api/routes/gen and
// held by make codegen-check). internal/wire hands it to the request validator of
// platform/httpserver (SEC-IN-1), so the server validates against exactly the contract it was
// generated from.
//
//go:embed openapi.gen.yaml
var Spec []byte
