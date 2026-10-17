// SPDX-License-Identifier: Apache-2.0

package hostos

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// The documented defaults of login.defs(5) for UID_MIN and UID_MAX.
const (
	DefaultUIDMin = 1000
	DefaultUIDMax = 60000
	// DarwinFirstPersonUID is the first UniqueID macOS gives a regular user.
	DarwinFirstPersonUID = 501
)

// ParseLoginDefs returns UID_MIN and UID_MAX of an /etc/login.defs content, each with its
// documented default when the key is missing or not a number.
func ParseLoginDefs(b []byte) (uidMin, uidMax int) {
	uidMin, uidMax = DefaultUIDMin, DefaultUIDMax
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 || strings.HasPrefix(f[0], "#") {
			continue
		}
		n, err := strconv.Atoi(f[1])
		if err != nil {
			continue
		}
		switch f[0] {
		case "UID_MIN":
			uidMin = n
		case "UID_MAX":
			uidMax = n
		}
	}
	return uidMin, uidMax
}

// PersonUID reports whether a uid belongs to a person account (HD9, D-M2-42-6): on Linux from
// UID_MIN to UID_MAX, on macOS from 501. nobody is never a person.
func PersonUID(goos, name string, uid, uidMin, uidMax int) bool {
	if name == "nobody" {
		return false
	}
	if goos == "darwin" {
		return uid >= DarwinFirstPersonUID
	}
	return uid >= uidMin && uid <= uidMax
}
