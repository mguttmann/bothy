// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

const (
	testOAuthURL = "https://oauth2.example.com/token"
	testEmail    = "push@example-chat.example.com"
	testKeyID    = "EXAMPLEKEY0001"
)

// accountJSON returns a service account JSON with the given fields; an empty value leaves a
// field out.
func accountJSON(t *testing.T, fields map[string]string) []byte {
	t.Helper()
	m := map[string]string{}
	for k, v := range fields {
		if v != "" {
			m[k] = v
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func pkcs8PEM(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestParseServiceAccount(t *testing.T) {
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ek, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	good := map[string]string{"type": "service_account", "client_email": testEmail, "private_key_id": testKeyID,
		"private_key": pkcs8PEM(t, rk), "token_uri": testOAuthURL, "project_id": "example-chat"}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for a, b := range good {
			m[a] = b
		}
		m[k] = v
		return m
	}
	sa, err := ParseServiceAccount(secrets.NewValue(accountJSON(t, good)), testOAuthURL)
	if err != nil {
		t.Fatalf("good account: %v", err)
	}
	if sa.clientEmail != testEmail || sa.keyID != testKeyID || !sa.key.Equal(rk) {
		t.Fatal("parsed fields differ")
	}
	if _, err := ParseServiceAccount(secrets.NewValue(accountJSON(t, with("token_uri", ""))), testOAuthURL); err != nil {
		t.Fatalf("absent token_uri: %v", err)
	}
	pkcs1 := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rk)}))
	bad := map[string][]byte{
		"type missing":     accountJSON(t, with("type", "")),
		"type wrong":       accountJSON(t, with("type", "authorized_user")),
		"email empty":      accountJSON(t, with("client_email", "")),
		"key id empty":     accountJSON(t, with("private_key_id", "")),
		"token_uri other":  accountJSON(t, with("token_uri", "https://token.example.com/token")),
		"ec key":           accountJSON(t, with("private_key", pkcs8PEM(t, ek))),
		"pkcs1 block":      accountJSON(t, with("private_key", pkcs1)),
		"two blocks":       accountJSON(t, with("private_key", pkcs8PEM(t, rk)+pkcs8PEM(t, rk))),
		"key missing":      accountJSON(t, with("private_key", "")),
		"broken json":      []byte(`{"type":"service_account",`),
		"empty":            nil,
		"not an object":    []byte(`["service_account"]`),
		"key not pem":      accountJSON(t, with("private_key", "not a key")),
		"pem with garbage": accountJSON(t, with("private_key", "x"+pkcs8PEM(t, rk))),
	}
	pemLines := strings.Split(pkcs8PEM(t, rk), "\n")
	for name, raw := range bad {
		t.Run(name, func(t *testing.T) {
			_, err := ParseServiceAccount(secrets.NewValue(raw), testOAuthURL)
			if err == nil {
				t.Fatal("parsed")
			}
			msg := err.Error()
			for _, s := range []string{testEmail, "token.example.com", testKeyID, "example-chat"} {
				if strings.Contains(msg, s) {
					t.Fatalf("error %q contains %q", msg, s)
				}
			}
			for _, l := range pemLines[1 : len(pemLines)-2] {
				if strings.Contains(msg, l) {
					t.Fatalf("error contains a line of the key")
				}
			}
		})
	}
}

func TestServiceAccountHasNoPrintedForm(t *testing.T) {
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	raw := accountJSON(t, map[string]string{"type": "service_account", "client_email": testEmail, "private_key_id": testKeyID,
		"private_key": pkcs8PEM(t, rk)})
	sa, err := ParseServiceAccount(secrets.NewValue(raw), testOAuthURL)
	if err != nil {
		t.Fatal(err)
	}
	j, _ := json.Marshal(sa)
	for _, s := range []string{fmt.Sprint(sa), fmt.Sprintf("%+v", sa), fmt.Sprintf("%#v", sa), string(j)} {
		if strings.Contains(s, testEmail) || strings.Contains(s, testKeyID) || strings.Contains(s, rk.D.String()[:12]) {
			t.Fatalf("printed form %q leaks", s)
		}
	}
}
