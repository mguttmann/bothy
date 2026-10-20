// SPDX-License-Identifier: Apache-2.0

// Package incoming holds the pure rules of incoming webhooks (docs/spec/11-erweiterungen.md
// section 8, M2-27 design notes 4, 5, 6c and 7): the classification of a request body as native
// or alertmanager, the body rules of both formats, the rendering of an Alertmanager notification
// into one message, the choice of the target thread and the context part of the request hash.
// It imports only the kernel core (02 section 5); the application layer gathers the facts and the
// configuration values and passes them in.
package incoming
