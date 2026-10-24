// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the search module: the Coordinator of
// docs/spec/14-search.md section 10, which implements search.Search over the outgoing port
// SearchIndex. The coordinator adds no rights logic of its own (05 rule 1): the visible channels
// come from Authorizer.Scope, every statement carries them, and every delivered hit passes
// authorize again in filter mode (14 4.3).
package app
