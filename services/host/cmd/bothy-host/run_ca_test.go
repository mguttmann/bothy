// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/state"
)

// TestServerRootsFromPairing (M2-43 red team): run trusts the CA that pair stored; without one
// the system roots; a broken one is an error.
func TestServerRootsFromPairing(t *testing.T) {
	st := state.Open(t.TempDir(), state.Plain{})
	if err := st.Ensure(); err != nil {
		t.Fatal(err)
	}
	if r, err := serverRoots(st); r != nil || err != nil {
		t.Fatalf("no CA: %v %v", r, err)
	}
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := st.WriteServerCA(ca); err != nil {
		t.Fatal(err)
	}
	r, err := serverRoots(st)
	if err != nil || r == nil {
		t.Fatalf("CA: %v", err)
	}
	if _, err := srv.Certificate().Verify(x509.VerifyOptions{Roots: r}); err != nil {
		t.Fatalf("the stored CA is not trusted: %v", err)
	}
	if err := st.WriteServerCA([]byte("broken")); err != nil {
		t.Fatal(err)
	}
	if _, err := serverRoots(st); err == nil {
		t.Fatal("a broken CA was accepted")
	}
}
