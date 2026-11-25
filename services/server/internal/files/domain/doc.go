// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the files module that need no storage: the key schema
// of docs/spec/12-files.md "4.4 Key schema" and the one key grammar that every object storage
// driver enforces, and the rules of part plans, session and file states, the scan outcome,
// purposes, images, delivery ranges and tickets, voice files, text extracts, cleanup ages and
// purge plans.
//
// Key builders take only an ids.ID, an int and the closed type Variant, so no file name, channel
// name or person can ever reach a key (SEC-UP-8). Errors wrap ErrInvalidKey and never repeat a
// key.
//
// Example:
//
//	id := ids.New()
//	orig, _ := domain.OriginalKey(id, 1)                  // f/<aa>/<bb>/<id>/orig-1
//	thumb, _ := domain.VariantKey(id, domain.VariantSmall) // f/<aa>/<bb>/<id>/small
//	err := domain.ValidateKey("../x")                       // wraps ErrInvalidKey
package domain
