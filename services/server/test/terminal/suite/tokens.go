// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// tokenOf returns the session token of a handshake (Design note 9, TM-T04): the token of another
// session (Of), a token with the own claims signed by a fresh Ed25519 key under a kid that is not
// in the JWKS (Forge foreign_kid), or one with the own claims and typ agent-invocation+jwt signed
// with the active terminal key of this throwaway stack (Forge invocation_typ). The token stays in
// memory.
func (w *instance) tokenOf(spec TokenSpec, own string) ([]byte, []string) {
	switch {
	case spec.Of != "":
		other, err := w.session(spec.Of)
		if err != nil {
			return nil, []string{err.Error()}
		}
		c, err := other.lastConn()
		if err != nil {
			return nil, []string{err.Error()}
		}
		return []byte(c.Token), nil
	case spec.Forge == "foreign_kid":
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, []string{err.Error()}
		}
		kid := make([]byte, 16)
		if _, err := rand.Read(kid); err != nil {
			return nil, []string{err.Error()}
		}
		t, err := Resign(own, priv, map[string]string{"kid": hex.EncodeToString(kid)})
		if err != nil {
			return nil, []string{err.Error()}
		}
		return t, nil
	case spec.Forge == "wdi_outside":
		// A server that bypasses the host service's list: the own claims with a wdi outside the
		// reported list of osu, signed with the active terminal key of the stack (TM-T29 (g)).
		kid, priv, err := w.activeTerminalKey()
		if err != nil {
			return nil, []string{err.Error()}
		}
		t, err := ResignClaims(own, priv, map[string]string{"kid": kid}, map[string]any{"wdi": 99})
		if err != nil {
			return nil, []string{err.Error()}
		}
		return t, nil
	case spec.Forge == "invocation_typ":
		kid, priv, err := w.activeTerminalKey()
		if err != nil {
			return nil, []string{err.Error()}
		}
		t, err := Resign(own, priv, map[string]string{"kid": kid, "typ": "agent-invocation+jwt"})
		if err != nil {
			return nil, []string{err.Error()}
		}
		return t, nil
	}
	return nil, []string{fmt.Sprintf("unknown token %+v", spec)}
}

// activeTerminalKey reads the kid of the active terminal key of the stack (server_signing_keys,
// read-only SQL) and its private key from the key store of api (M1-32: one PEM file
// <purpose>-<kid>.pem in secrets.signing_key_dir), copied into a file of mode 0600 of the test's
// temp dir. The key of a throwaway stack stands for a faulty server.
func (w *instance) activeTerminalKey() (string, ed25519.PrivateKey, error) {
	rows, err := w.env.Stack.SQL(w.ctx, "SELECT kid FROM server_signing_keys WHERE purpose = 'terminal' AND state = 'active'", false)
	if err != nil {
		return "", nil, err
	}
	if len(rows) != 1 {
		return "", nil, fmt.Errorf("%d active terminal keys", len(rows))
	}
	kid := fmt.Sprint(rows[0]["kid"])
	dst := filepath.Join(w.t.TempDir(), "terminal.pem")
	if err := w.env.Stack.CopyFrom(w.ctx, "api", "/var/lib/bothy/signing-keys/terminal-"+kid+".pem", dst); err != nil {
		return "", nil, err
	}
	b, err := os.ReadFile(dst) //nolint:gosec // G304: the copied key file in the test's temp dir
	if err != nil {
		return "", nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return "", nil, errors.New("the terminal key file is no PEM block")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", nil, err
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return "", nil, errors.New("the terminal key is no Ed25519 key")
	}
	return kid, priv, nil
}

// Resign returns the compact JWS token with the header members of set replaced and the signature
// made with priv (EdDSA); the claims stay as they are.
//
// Errors: a token that is no compact JWS.
func Resign(token string, priv ed25519.PrivateKey, set map[string]string) ([]byte, error) {
	return ResignClaims(token, priv, set, nil)
}

// ResignClaims is Resign that also replaces the claims of claims.
//
// Errors: a token that is no compact JWS.
func ResignClaims(token string, priv ed25519.PrivateKey, set map[string]string, claims map[string]any) ([]byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("the session token is no compact JWS")
	}
	hb, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	var header map[string]any
	if err := json.Unmarshal(hb, &header); err != nil {
		return nil, err
	}
	for k, v := range set {
		header[k] = v
	}
	nh, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	payload := parts[1]
	if len(claims) > 0 {
		pb, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}
		var c map[string]any
		if err := json.Unmarshal(pb, &c); err != nil {
			return nil, err
		}
		for k, v := range claims {
			c[k] = v
		}
		nb, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		payload = base64.RawURLEncoding.EncodeToString(nb)
	}
	input := base64.RawURLEncoding.EncodeToString(nh) + "." + payload
	sig := ed25519.Sign(priv, []byte(input))
	return []byte(input + "." + base64.RawURLEncoding.EncodeToString(sig)), nil
}
