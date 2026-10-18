// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the chat module: the space and sidebar part of chat.Structure
// (M1-52). Every use case loads its resource, authorizes the caller with the action of
// docs/spec/05-actions.md section 4.5 and writes in one unit of work with audit and the durable
// event; space.archive runs as a declared rights change.
package app
