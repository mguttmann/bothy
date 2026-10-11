// SPDX-License-Identifier: Apache-2.0

// Package rest stands in for the server interface generated from the OpenAPI document.
package rest

// Server is the generated strict server interface.
type Server interface {
	Ping() string
}
