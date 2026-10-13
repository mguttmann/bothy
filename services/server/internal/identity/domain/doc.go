// SPDX-License-Identifier: Apache-2.0

// Package domain holds the entities and value objects of the identity module: principal kinds
// and states, handles and display names, and the entities Principal, User and Owner of
// docs/spec/03-data-model.md section 4.2. Constructors validate; a value of Handle or
// DisplayName obtained from them always satisfies the rules of 03 and SEC-IN-4.
//
// Validation errors are kernel/errs errors of code validation_failed with one field error
// (pointer /handle, /display_name or /reason) whose code names the rule (required, too_long,
// pattern or a textcheck rule). They never repeat the rejected input.
//
// Example:
//
//	h, err := domain.NewHandle("alex", 32)
//	d, err := domain.NewDisplayName("Alex", 64)
//	p, err := domain.NewPrincipal(ids.New(), domain.KindUser, h, d)
package domain
