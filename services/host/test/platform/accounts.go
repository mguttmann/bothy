// SPDX-License-Identifier: Apache-2.0

package platform

// The accounts the suite creates next to the service user: the session users A, B and a third,
// and a person of the host who is no session user (TM-T24). They live in this untagged file so
// make check holds them to the bound of Windows (accounts_test.go).
const (
	userA = "bothy-s-alex"
	userB = "bothy-s-sam"
	userC = "bothy-s-robin"
	// personUser is the account whose home a session user must not list (HD9).
	personUser = "bothy-person"
)

// winAccountNameMax is the longest name Windows accepts for a local user account ("net user":
// "The name of the user account can have as many as 20 characters",
// https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2012-r2-and-2012/cc771865(v=ws.11),
// retrieved 2026-11-11).
const winAccountNameMax = 20
