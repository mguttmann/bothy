// SPDX-License-Identifier: Apache-2.0

package actor

// Kind is the kind of the principal of a request (docs/spec/07-rechte.md section 3, field
// `kind`). The values are the stable strings of the API (`PrincipalKind` in
// docs/api/openapi.yaml) plus the two kinds without a row in `principals`.
type Kind string

const (
	// KindUser is a person (a row in `principals` and `users`).
	KindUser Kind = "user"
	// KindAgent is an agent principal.
	KindAgent Kind = "agent"
	// KindApp is an app principal.
	KindApp Kind = "app"
	// KindAnonymous is a request without an authenticated principal.
	KindAnonymous Kind = "anonymous"
	// KindHost is a host machine signed in with its host key (docs/spec/24-terminal.md section
	// 4.1); it has no row in `principals`.
	KindHost Kind = "host"
)

// Valid reports whether k is one of the defined kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindUser, KindAgent, KindApp, KindAnonymous, KindHost:
		return true
	}
	return false
}

// IsPrincipal reports whether k is a kind with a row in `principals` (user, agent, app), the
// values of the column `principals.kind`.
func (k Kind) IsPrincipal() bool {
	return k == KindUser || k == KindAgent || k == KindApp
}
