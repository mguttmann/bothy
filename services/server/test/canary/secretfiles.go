// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"log"
	"log/slog"
	"strings"
)

// SecretFileForms returns the texts of a secret file the scan looks for: the trimmed content
// (Scan adds its base64 and hex forms), the bytes a hex or base64 text decodes to, for PEM every
// body line and the private key material, for a JWK the private member d.
func SecretFileForms(b []byte) []string {
	text := strings.TrimSpace(string(b))
	out := []string{text}
	if raw, err := hex.DecodeString(text); err == nil && len(raw) > 0 {
		out = append(out, string(raw))
	}
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if raw, err := enc.DecodeString(text); err == nil && len(raw) > 0 {
			out = append(out, string(raw))
			break
		}
	}
	rest := b
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		for line := range strings.SplitSeq(string(pem.EncodeToMemory(block)), "\n") {
			if line != "" && !strings.HasPrefix(line, "-----") {
				out = append(out, line)
			}
		}
		out = append(out, privateMaterial(block.Bytes)...)
	}
	var jwk struct {
		D string `json:"d"`
	}
	if json.Unmarshal(b, &jwk) == nil && jwk.D != "" {
		out = append(out, jwk.D)
	}
	return out
}

// privateMaterial returns the private scalar of a PKCS #8, SEC 1 or PKCS #1 key as raw bytes
// text and as base64url (JWK form).
func privateMaterial(der []byte) []string {
	var key any
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		key = k
	} else if k, err := x509.ParseECPrivateKey(der); err == nil {
		key = k
	} else if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		key = k
	}
	var raw []byte
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		b, err := k.Bytes()
		if err != nil {
			return nil
		}
		raw = b
	case ed25519.PrivateKey:
		raw = k.Seed()
	case *rsa.PrivateKey:
		raw = k.D.Bytes()
	default:
		return nil
	}
	return []string{string(raw), base64.RawURLEncoding.EncodeToString(raw)}
}

// swapStdLoggers points the standard logger and slog's default logger at w and returns the
// function that restores both.
func swapStdLoggers(w io.Writer) func() {
	prevOut, prevFlags, prevDefault := log.Writer(), log.Flags(), slog.Default()
	log.SetOutput(w)
	slog.SetDefault(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return func() {
		slog.SetDefault(prevDefault)
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	}
}
