// SPDX-License-Identifier: Apache-2.0

// Package fingerprint computes the key fingerprint of a host (docs/spec/24-terminal.md 4.3):
// base32_crockford(SHA-256("bothy-host-fp/1" || noise_public_key || auth_public_key_raw)),
// shown in groups of four characters. The server (M2-41), the host service (M2-42) and the
// Dart package bothy_noise compute the same value; testdata/noise/fingerprint.json holds the
// shared vectors. The package does no I/O and depends only on the standard library.
package fingerprint
