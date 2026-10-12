// SPDX-License-Identifier: Apache-2.0

// Package instance is the port of the instance module: instance identity, sync epoch, minimum
// server version, settings and branding, feature flags, server signing keys with JWKS and
// rotation, and the sequence counters. Other modules import only this package
// (docs/spec/02-architecture.md, "Layers within a module").
package instance
