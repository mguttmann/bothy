// SPDX-License-Identifier: Apache-2.0

// Package passwordpolicy holds the local list of common passwords and the NFC normalisation of
// passwords (identity/app CommonPasswords and PasswordNormalizer, docs/spec/08-identity.md
// section 2.5, brief M1-39). The list is embedded as common_passwords.txt: per entry the first 16
// hex characters of SHA-256(lowercase(NFC(password))), sorted and unique, so the repository holds
// no word list. Source, commit, licence and the generation command are in README.md. Contains
// hashes the candidate the same way and searches the sorted list.
//
// Example:
//
//	l := passwordpolicy.Default()
//	nfc := l.NFC(password)
//	listed := l.Contains(nfc)
package passwordpolicy

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed" // the hashed list
	"encoding/binary"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/text/unicode/norm"
)

//go:embed common_passwords.txt
var commonPasswords []byte

// List implements app.CommonPasswords and app.PasswordNormalizer. It is safe for concurrent use.
type List struct {
	prefixes []uint64
}

// Default returns the list of the embedded file, parsed once. A malformed line is a build error of
// the repository that TestCommonPasswordsFile catches; it is skipped here.
var Default = sync.OnceValue(func() *List {
	return parse(commonPasswords)
})

// parse reads one 16-character hex prefix per line.
func parse(data []byte) *List {
	l := &List{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		v, err := strconv.ParseUint(strings.TrimSpace(sc.Text()), 16, 64)
		if err == nil {
			l.prefixes = append(l.prefixes, v)
		}
	}
	slices.Sort(l.prefixes)
	return l
}

// NFC implements app.PasswordNormalizer.
func (*List) NFC(password string) string { return norm.NFC.String(password) }

// Contains implements app.CommonPasswords: the candidate is normalised to NFC and lowercased
// before hashing, like the generated entries.
func (l *List) Contains(password string) bool {
	sum := sha256.Sum256([]byte(strings.ToLower(norm.NFC.String(password))))
	_, found := slices.BinarySearch(l.prefixes, binary.BigEndian.Uint64(sum[:8]))
	return found
}
