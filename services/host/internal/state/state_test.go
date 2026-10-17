// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func newDir(t *testing.T) *Dir {
	t.Helper()
	d := Open(filepath.Join(t.TempDir(), "state"), Plain{})
	if err := d.Ensure(); err != nil {
		t.Fatal(err)
	}
	return d
}

func testKeys(t *testing.T) Keys {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k := Keys{AuthPrivate: priv}
	k.NoisePrivate[0] = 7
	return k
}

func TestLayoutModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses the protected DACL of the directory (TestPlatform_DPAPIAndModes)")
	}
	d := newDir(t)
	if err := d.WriteKeys(testKeys(t)); err != nil {
		t.Fatal(err)
	}
	if err := d.WritePairing(Pairing{ServerURL: "https://chat.example.com", HostID: "h", InstanceID: "i", Fingerprint: "F"}); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteBindings(map[string]string{"bothy-s-alex": "p1"}); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteDeclaration(DeclOrganization); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(d.Path())
	if err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode %v %v", fi.Mode().Perm(), err)
	}
	entries, _ := os.ReadDir(d.Path())
	if len(entries) != 4 {
		t.Errorf("entries %v (temporary files left?)", entries)
	}
	for _, e := range entries {
		fi, _ := e.Info()
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %v", e.Name(), fi.Mode().Perm())
		}
	}
	if d.File(AdminSocket) != filepath.Join(d.Path(), "admin.sock") {
		t.Error("admin socket path")
	}
}

func TestAtomicWrite(t *testing.T) {
	d := newDir(t)
	if err := d.WriteDeclaration(DeclNone); err != nil {
		t.Fatal(err)
	}
	crash := errors.New("crash before rename")
	d.rename = func(string, string) error { return crash }
	if err := d.WriteDeclaration(DeclPersonal); !errors.Is(err, crash) {
		t.Fatalf("err %v", err)
	}
	if got, err := d.ReadDeclaration(); err != nil || got != DeclNone {
		t.Errorf("after the crash: %q %v", got, err)
	}
	entries, _ := os.ReadDir(d.Path())
	if len(entries) != 1 {
		t.Errorf("temporary file left: %v", entries)
	}
	bad := Open(filepath.Join(t.TempDir(), "missing"), Plain{})
	if err := bad.WriteDeclaration(DeclNone); err == nil {
		t.Error("write into a missing directory succeeded")
	}
}

func TestKeysRoundTrip(t *testing.T) {
	d := newDir(t)
	if _, err := d.ReadKeys(); !errors.Is(err, ErrNotPaired) {
		t.Fatalf("missing keys: %v", err)
	}
	k := testKeys(t)
	if err := d.WriteKeys(k); err != nil {
		t.Fatal(err)
	}
	got, err := d.ReadKeys()
	if err != nil || got.NoisePrivate != k.NoisePrivate || !bytes.Equal(got.AuthPrivate, k.AuthPrivate) {
		t.Fatalf("keys differ: %v", err)
	}
	if err := d.WriteKeys(Keys{}); err == nil {
		t.Error("empty login key accepted")
	}
	if err := os.WriteFile(d.File(KeysFile), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ReadKeys(); err == nil {
		t.Error("broken key file accepted")
	}
	if err := os.WriteFile(d.File(KeysFile), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ReadKeys(); err == nil {
		t.Error("garbage key file accepted")
	}
}

func TestPairingRecordHasNoPrivateMember(t *testing.T) {
	d := newDir(t)
	if _, err := d.ReadPairing(); !errors.Is(err, ErrNotPaired) {
		t.Fatal(err)
	}
	p := Pairing{ServerURL: "https://chat.example.com", HostID: "0192f4a0-7b1c-7a11-8c3e-4f5a6b7c8d9e", InstanceID: "0192f4a0-7b1c-7000-8a00-000000000001", Fingerprint: "F0CC"}
	if err := d.WritePairing(p); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(d.File(PairingFile))
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for k := range m {
		if k != "server_url" && k != "host_id" && k != "instance_id" && k != "key_fingerprint" {
			t.Errorf("member %q", k)
		}
	}
	if got, err := d.ReadPairing(); err != nil || got != p {
		t.Errorf("read %+v %v", got, err)
	}
	if err := os.WriteFile(d.File(PairingFile), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ReadPairing(); err == nil {
		t.Error("broken pairing accepted")
	}
}

func TestBindings(t *testing.T) {
	d := newDir(t)
	m, err := d.ReadBindings()
	if err != nil || len(m) != 0 {
		t.Fatalf("empty: %v %v", m, err)
	}
	if err := d.WriteBindings(map[string]string{"bothy-s-alex": "0192f4a0-0000-7000-8000-000000000001"}); err != nil {
		t.Fatal(err)
	}
	m, err = d.ReadBindings()
	if err != nil || m["bothy-s-alex"] == "" {
		t.Errorf("read %v %v", m, err)
	}
	if err := os.WriteFile(d.File(BindingsFile), []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ReadBindings(); err == nil {
		t.Error("wrong JSON accepted")
	}
}

func TestDeclaration(t *testing.T) {
	d := newDir(t)
	if got, err := d.ReadDeclaration(); err != nil || got != DeclUnknown {
		t.Fatalf("never stored: %q %v", got, err)
	}
	for _, v := range []string{DeclNone, DeclPersonal, DeclOrganization} {
		if err := d.WriteDeclaration(v); err != nil {
			t.Fatal(err)
		}
		if got, _ := d.ReadDeclaration(); got != v {
			t.Errorf("got %q want %q", got, v)
		}
	}
	if err := d.WriteDeclaration(DeclUnknown); err == nil {
		t.Error("unknown stored")
	}
	if err := os.WriteFile(d.File(DeclarationFile), []byte(`{"llm_credentials":"shared"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := d.ReadDeclaration(); err == nil || got != DeclUnknown {
		t.Errorf("foreign value: %q %v", got, err)
	}
}
