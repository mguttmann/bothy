// SPDX-License-Identifier: Apache-2.0

// Package api holds the code generated from the API contract in docs/api (ADR 0013): rest
// (oapi-codegen: models, std-http and strict server of the enabled operations) and routes (route
// table, action catalogue, glue). It has no code of its own; this file only carries the generate
// directives, which `go generate ./...` in services/server runs and `make codegen-check` holds
// (R-API-18). How to enable an operation: services/server/api/README.md.
package api

// The real surface: docs/api/openapi.yaml filtered to the enable list operations.yaml.
//go:generate go run ./routes/gen -spec ../../../docs/api/openapi.yaml -catalog ../../../docs/spec/actions.yaml -list operations.yaml -codegen-config rest/oapi-codegen.yaml -rest-dir rest -routes-dir routes -rest-pkg github.com/OWNER/bothy/services/server/api/rest -asyncapi ../../../docs/api/asyncapi.yaml

// The fixture of the generator's tests: three enabled operations, generated the same way.
//go:generate go run ./routes/gen -spec routes/gen/testdata/fixture/openapi.yaml -catalog routes/gen/testdata/fixture/actions.yaml -list routes/gen/testdata/fixture/operations.yaml -codegen-config rest/oapi-codegen.yaml -rest-dir routes/internal/fixture/rest -routes-dir routes/internal/fixture/routes -rest-pkg github.com/OWNER/bothy/services/server/api/routes/internal/fixture/rest -core-pkg github.com/OWNER/bothy/services/server/api/routes
