// SPDX-License-Identifier: Apache-2.0

//go:build race

package httpapi_test

// raceEnabled reports a build with the race detector, which slows the instrumented Argon2id code
// many times over (see TestIDT21_UnknownNameTimingEqualsWrongPassword).
const raceEnabled = true
