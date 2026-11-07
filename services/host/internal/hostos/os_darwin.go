// SPDX-License-Identifier: Apache-2.0

package hostos

// PtraceScope implements Probe; macOS has no Yama.
func (p UnixProbe) PtraceScope() (int, error) { return -1, nil }

// Platform returns the Users, Security and Probe of macOS.
func Platform(r Runner) (Users, Security, Probe) {
	u := DarwinUsers{Runner: r}
	return u, DarwinSecurity{Runner: r, Users: u}, UnixProbe{Runner: r}
}
