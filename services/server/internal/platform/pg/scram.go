// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"
)

// The SCRAM-SHA-256 parameters of the verifiers Bootstrap computes (docs/DECISIONS.md M1-12):
// 4096 iterations is the default of PostgreSQL's scram_iterations and the minimum of RFC 7677,
// so a login costs what it costs with a password PostgreSQL hashed itself; 16 bytes of salt is
// what PostgreSQL generates.
const (
	scramIterations = 4096
	scramSaltBytes  = 16
)

// verifierPattern is the form PostgreSQL stores and accepts as an already hashed password:
// SCRAM-SHA-256$<iterations>:<salt>$<StoredKey>:<ServerKey>, base64 with padding.
var verifierPattern = regexp.MustCompile(`^SCRAM-SHA-256\$[1-9][0-9]*:[A-Za-z0-9+/]+={0,2}\$[A-Za-z0-9+/]{43}=:[A-Za-z0-9+/]{43}=$`)

// newSCRAMVerifier returns the verifier of password with a fresh random salt. The password
// itself never leaves the process; only the verifier goes to the server.
func newSCRAMVerifier(password []byte) (string, error) {
	salt := make([]byte, scramSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("pg: salt for a SCRAM verifier: %w", err)
	}
	return scramVerifier(password, salt, scramIterations)
}

// scramVerifier computes the SCRAM-SHA-256 verifier of password (RFC 5802 section 3, RFC 7677):
// SaltedPassword = PBKDF2-HMAC-SHA-256(password, salt, iterations), StoredKey =
// SHA-256(HMAC(SaltedPassword, "Client Key")), ServerKey = HMAC(SaltedPassword, "Server Key").
// The password is used as it is: checkPasswords allows only printable ASCII, for which
// SASLprep, which PostgreSQL and the clients apply, changes nothing.
func scramVerifier(password, salt []byte, iterations int) (string, error) {
	salted, err := pbkdf2.Key(sha256.New, string(password), salt, iterations, sha256.Size)
	if err != nil {
		return "", fmt.Errorf("pg: SCRAM key derivation: %w", err)
	}
	defer clear(salted)
	clientKey := hmacSHA256(salted, "Client Key")
	defer clear(clientKey)
	storedKey := sha256.Sum256(clientKey)
	serverKey := hmacSHA256(salted, "Server Key")
	enc := base64.StdEncoding
	return fmt.Sprintf("SCRAM-SHA-256$%d:%s$%s:%s", iterations,
		enc.EncodeToString(salt), enc.EncodeToString(storedKey[:]), enc.EncodeToString(serverKey)), nil
}

func hmacSHA256(key []byte, msg string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(msg))
	return m.Sum(nil)
}
