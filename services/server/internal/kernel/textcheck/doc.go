// SPDX-License-Identifier: Apache-2.0

// Package textcheck checks display texts as SEC-IN-4 requires: invalid UTF-8, control characters
// and bidi control characters; it does not check lengths. It is part of the kernel itself and used
// by the domain packages of all modules with display texts.
package textcheck
