// SPDX-License-Identifier: Apache-2.0

//go:build unix

package stack_test

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"

	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
)

// tarEntry is what the bundle must preserve of one _store entry.
type tarEntry struct {
	dir     bool
	mode    int64
	content string
}

// store prepares a _store through Init with every kind of entry and returns its path and the
// entries read back from disk (names as in the bundle, directories with a trailing slash).
func store(t *testing.T) (string, map[string]tarEntry) {
	t.Helper()
	target, source := dirs(t)
	put(t, source, "apns_key.p8", "EXAMPLE-apns", 0o600)
	put(t, source, "oidc/alpha_client_secret", "EXAMPLE-oidc", 0o600)
	put(t, source, "offsite/ssh/id_ed25519", "EXAMPLE-offsite", 0o600)
	mustInit(t, opts(target, source))
	dir := filepath.Join(target, "_store")
	want := map[string]tarEntry{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if d.IsDir() {
			want[name+"/"] = tarEntry{dir: true, mode: int64(info.Mode().Perm())}
			return nil
		}
		want[name] = tarEntry{mode: int64(info.Mode().Perm()), content: read(t, dir, rel)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir, want
}

func identity(t *testing.T) *age.X25519Identity {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func notTerminal(io.Writer) bool { return false }

// untar decrypts a bundle and returns its entries in stream order.
func untar(t *testing.T, bundle []byte, armored bool, id age.Identity) ([]string, map[string]tarEntry) {
	t.Helper()
	var src io.Reader = bytes.NewReader(bundle)
	if armored {
		src = armor.NewReader(src)
	}
	plain, err := age.Decrypt(src, id)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	tr := tar.NewReader(plain)
	var names []string
	got := map[string]tarEntry{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		if hdr.ModTime.Unix() != 0 || hdr.Uid != 0 || hdr.Gid != 0 || hdr.Uname != "" || hdr.Gname != "" {
			t.Errorf("%s: mtime %v uid %d gid %d uname %q gname %q, want zero and no names",
				hdr.Name, hdr.ModTime, hdr.Uid, hdr.Gid, hdr.Uname, hdr.Gname)
		}
		if strings.HasPrefix(hdr.Name, "/") || strings.Contains(hdr.Name, "..") {
			t.Errorf("%s is not a relative name", hdr.Name)
		}
		e := tarEntry{dir: hdr.Typeflag == tar.TypeDir, mode: hdr.Mode}
		if hdr.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			e.content = string(data)
		} else if !e.dir {
			t.Errorf("%s has type %q", hdr.Name, hdr.Typeflag)
		}
		names = append(names, hdr.Name)
		got[hdr.Name] = e
	}
	return names, got
}

func compareEntries(t *testing.T, got, want map[string]tarEntry) {
	t.Helper()
	for name, w := range want {
		if g, ok := got[name]; !ok {
			t.Errorf("bundle lacks %s", name)
		} else if g != w {
			t.Errorf("%s: bundle %+v, _store %+v", name, g, w)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("bundle has extra entry %s", name)
		}
	}
}

func TestExportRoundTrip(t *testing.T) {
	t.Parallel()
	for _, armored := range []bool{false, true} {
		t.Run(map[bool]string{false: "binary", true: "armor"}[armored], func(t *testing.T) {
			t.Parallel()
			from, want := store(t)
			put(t, from, ".bothy-tmp-leftover", "EXAMPLE-partial", 0o600) // never in the bundle
			id := identity(t)
			var out bytes.Buffer
			err := stack.Export(t.Context(), stack.ExportOptions{
				From: from, Recipients: []string{id.Recipient().String()}, Armor: armored,
				Stdout: &out, IsTerminal: notTerminal,
			})
			if err != nil {
				t.Fatalf("Export: %v", err)
			}
			if armored != strings.HasPrefix(out.String(), "-----BEGIN AGE ENCRYPTED FILE-----") {
				t.Errorf("armor %v, output starts with %q", armored, out.String()[:min(out.Len(), 40)])
			}
			for _, e := range want {
				if e.content != "" && bytes.Contains(out.Bytes(), []byte(e.content)) {
					t.Fatal("bundle contains plaintext")
				}
			}
			names, got := untar(t, out.Bytes(), armored, id)
			compareEntries(t, got, want)
			if !slices.IsSorted(names) {
				t.Errorf("bundle names are not sorted: %v", names)
			}
			for _, must := range []string{"apns_key.p8", "oidc/", "oidc/alpha_client_secret", "offsite/ssh/", "offsite/ssh/id_ed25519", "token_pepper"} {
				if _, ok := got[must]; !ok {
					t.Errorf("bundle lacks %s", must)
				}
			}
			if got["token_pepper"].mode != 0o400 || got["oidc/"].mode != 0o700 {
				t.Errorf("modes not kept: token_pepper %o, oidc/ %o", got["token_pepper"].mode, got["oidc/"].mode)
			}
		})
	}
}

func TestExportTwoRecipients(t *testing.T) {
	t.Parallel()
	from, want := store(t)
	alex, sam := identity(t), identity(t)
	var out bytes.Buffer
	err := stack.Export(t.Context(), stack.ExportOptions{
		From: from, Recipients: []string{alex.Recipient().String(), sam.Recipient().String()},
		Stdout: &out, IsTerminal: notTerminal,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	for _, id := range []*age.X25519Identity{alex, sam} {
		_, got := untar(t, out.Bytes(), false, id)
		compareEntries(t, got, want)
	}
}

func TestExportRefusesWithoutRecipient(t *testing.T) {
	t.Parallel()
	from, _ := store(t)
	output := filepath.Join(t.TempDir(), "bundle.age")
	for _, o := range []stack.ExportOptions{
		{From: from, Stdout: &bytes.Buffer{}, IsTerminal: notTerminal},
		{From: from, Output: output, Recipients: []string{}},
	} {
		err := stack.Export(t.Context(), o)
		if !errors.Is(err, stack.ErrUsage) {
			t.Fatalf("Export error %v, want ErrUsage", err)
		}
		if b, ok := o.Stdout.(*bytes.Buffer); ok && b.Len() != 0 {
			t.Error("Export wrote output without a recipient")
		}
	}
	if exists(t, output) {
		t.Error("Export created the output file without a recipient")
	}
}

func TestExportRejectsInvalidRecipient(t *testing.T) {
	t.Parallel()
	from, _ := store(t)
	valid := identity(t).Recipient().String()
	// An identity pasted as recipient is the likely mistake; it must never be echoed.
	secretKey := identity(t).String()
	for _, bad := range []string{"age1EXAMPLEinvalid", secretKey, "", "ssh-ed25519 AAAAEXAMPLE"} {
		output := filepath.Join(t.TempDir(), "bundle.age")
		err := stack.Export(t.Context(), stack.ExportOptions{From: from, Recipients: []string{valid, bad}, Output: output})
		if !errors.Is(err, stack.ErrUsage) {
			t.Fatalf("Export(%d chars) error %v, want ErrUsage", len(bad), err)
		}
		if bad != "" && strings.Contains(err.Error(), bad) {
			t.Error("error message echoes the recipient")
		}
		if strings.Contains(err.Error(), "AGE-SECRET-KEY") {
			t.Error("error message shows a secret key")
		}
		if exists(t, output) {
			t.Error("Export created the output file")
		}
	}
}

func TestExportRefusesTerminalWithoutArmor(t *testing.T) {
	t.Parallel()
	from, want := store(t)
	id := identity(t)
	terminal := func(io.Writer) bool { return true }
	var out bytes.Buffer
	err := stack.Export(t.Context(), stack.ExportOptions{
		From: from, Recipients: []string{id.Recipient().String()}, Stdout: &out, IsTerminal: terminal,
	})
	if !errors.Is(err, stack.ErrUsage) {
		t.Fatalf("Export error %v, want ErrUsage", err)
	}
	if out.Len() != 0 {
		t.Error("Export wrote binary output to a terminal")
	}
	err = stack.Export(t.Context(), stack.ExportOptions{
		From: from, Recipients: []string{id.Recipient().String()}, Output: "-", Armor: true,
		Stdout: &out, IsTerminal: terminal,
	})
	if err != nil {
		t.Fatalf("Export with armor: %v", err)
	}
	_, got := untar(t, out.Bytes(), true, id)
	compareEntries(t, got, want)
}

func TestExportDefaultTerminalCheckAcceptsFiles(t *testing.T) {
	t.Parallel()
	from, want := store(t)
	id := identity(t)
	path := filepath.Join(t.TempDir(), "stdout")
	f, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	err = stack.Export(t.Context(), stack.ExportOptions{From: from, Recipients: []string{id.Recipient().String()}, Stdout: f})
	if cerr := f.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if err != nil {
		t.Fatalf("Export to a file as stdout: %v", err)
	}
	_, got := untar(t, []byte(read(t, filepath.Dir(path), "stdout")), false, id)
	compareEntries(t, got, want)
}

func TestExportOutputFileMode0600AndNoOverwrite(t *testing.T) {
	t.Parallel()
	from, want := store(t)
	id := identity(t)
	output := filepath.Join(t.TempDir(), "bundle.age")
	o := stack.ExportOptions{From: from, Recipients: []string{id.Recipient().String()}, Output: output}
	if err := stack.Export(t.Context(), o); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if got := perm(t, output); got != 0o600 {
		t.Errorf("bundle mode %v, want 0600", got)
	}
	first := read(t, filepath.Dir(output), "bundle.age")
	_, got := untar(t, []byte(first), false, id)
	compareEntries(t, got, want)

	err := stack.Export(t.Context(), o)
	if err == nil {
		t.Fatal("second Export overwrote the bundle")
	}
	if errors.Is(err, stack.ErrUsage) {
		t.Errorf("existing output is a usage error (exit 2), want a runtime error: %v", err)
	}
	if read(t, filepath.Dir(output), "bundle.age") != first {
		t.Error("the existing bundle changed")
	}
}

func TestExportRejectsNonRegularEntry(t *testing.T) {
	t.Parallel()
	from, _ := store(t)
	if err := os.Symlink(filepath.Join(from, "token_pepper"), filepath.Join(from, "linked")); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "bundle.age")
	err := stack.Export(t.Context(), stack.ExportOptions{From: from, Recipients: []string{identity(t).Recipient().String()}, Output: output})
	if err == nil {
		t.Fatal("Export accepted a symlink in the store")
	}
	if !strings.Contains(err.Error(), "linked") {
		t.Errorf("error %q does not name the entry", err)
	}
	if exists(t, output) {
		t.Error("Export left an output file behind")
	}
}

func TestExportRemovesOutputFileOnFailure(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root: an unreadable file cannot be simulated")
	}
	from, _ := store(t)
	chmod(t, filepath.Join(from, "token_pepper"), 0o000)
	defer chmod(t, filepath.Join(from, "token_pepper"), 0o400)
	output := filepath.Join(t.TempDir(), "bundle.age")
	err := stack.Export(t.Context(), stack.ExportOptions{From: from, Recipients: []string{identity(t).Recipient().String()}, Output: output})
	if err == nil {
		t.Fatal("Export succeeded with an unreadable file")
	}
	if exists(t, output) {
		t.Error("Export left a partial bundle behind")
	}
}

func TestExportRequiresFromDirectory(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	put(t, base, "file", "EXAMPLE", 0o600)
	recipient := identity(t).Recipient().String()
	for _, from := range []string{"", filepath.Join(base, "absent"), filepath.Join(base, "file")} {
		var out bytes.Buffer
		err := stack.Export(t.Context(), stack.ExportOptions{From: from, Recipients: []string{recipient}, Stdout: &out, IsTerminal: notTerminal})
		if !errors.Is(err, stack.ErrUsage) {
			t.Errorf("Export(From %q) error %v, want ErrUsage", from, err)
		}
		if out.Len() != 0 {
			t.Errorf("Export(From %q) wrote output", from)
		}
	}
}

func TestExportHonoursCanceledContext(t *testing.T) {
	t.Parallel()
	from, _ := store(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	output := filepath.Join(t.TempDir(), "bundle.age")
	if err := stack.Export(ctx, stack.ExportOptions{From: from, Recipients: []string{identity(t).Recipient().String()}, Output: output}); err == nil {
		t.Fatal("Export ran with a canceled context")
	}
	if exists(t, output) {
		t.Error("Export left an output file behind")
	}
}

func TestExportErrorsEscapeControlCharacters(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root: an unreadable file cannot be simulated")
	}
	from := t.TempDir()
	put(t, from, "offsite/forged\nwarning: all good", "EXAMPLE-offsite", 0o000)
	var out bytes.Buffer
	err := stack.Export(t.Context(), stack.ExportOptions{
		From: from, Recipients: []string{identity(t).Recipient().String()}, Stdout: &out, IsTerminal: notTerminal,
	})
	if err == nil {
		t.Fatal("Export read an unreadable file")
	}
	if strings.ContainsAny(err.Error(), "\n\r") {
		t.Errorf("error message contains a raw control character: %q", err.Error())
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("error %v does not wrap the cause", err)
	}
}

// TestExportRefusesFilesWithoutCatalogEntry holds that export bundles only catalog files: the
// bundle is what init adopts as --source (disaster-recovery.md Fall B), and init refuses any
// other file, so a bundle of the volume root or of a _store with a stray file would fail exactly
// at the restore. The error names the files and the output file is not created.
func TestExportRefusesFilesWithoutCatalogEntry(t *testing.T) {
	t.Parallel()
	dir, _ := store(t)
	volume := filepath.Dir(dir)
	stray := t.TempDir()
	put(t, stray, "db_app_password", "EXAMPLE-app", 0o400)
	put(t, stray, "bundle.age", "EXAMPLE-old-bundle", 0o400)
	for from, name := range map[string]string{volume: "server/db_app_password", stray: "bundle.age"} {
		out := filepath.Join(t.TempDir(), "bundle.age")
		err := stack.Export(t.Context(), stack.ExportOptions{
			From: from, Recipients: []string{identity(t).Recipient().String()}, Output: out, IsTerminal: notTerminal,
		})
		if !errors.Is(err, stack.ErrUsage) || !strings.Contains(err.Error(), name) {
			t.Errorf("export of %s: error %v, want ErrUsage naming %s", from, err, name)
		}
		if _, statErr := os.Stat(out); !errors.Is(statErr, fs.ErrNotExist) {
			t.Errorf("export of %s created the output file: %v", from, statErr)
		}
	}
}

// TestExportBundleRestoresThroughInit closes the escrow round trip of disaster-recovery.md Fall
// B: a bundle, decrypted and unpacked as the source of a new volume, is adopted entry by entry,
// and the new _store equals the old one.
func TestExportBundleRestoresThroughInit(t *testing.T) {
	t.Parallel()
	dir, want := store(t)
	id := identity(t)
	var bundle bytes.Buffer
	if err := stack.Export(t.Context(), stack.ExportOptions{
		From: dir, Recipients: []string{id.Recipient().String()}, Stdout: &bundle, IsTerminal: notTerminal,
	}); err != nil {
		t.Fatal(err)
	}
	_, entries := untar(t, bundle.Bytes(), false, id)
	target, source := dirs(t)
	for name, e := range entries {
		if !e.dir {
			put(t, source, name, e.content, 0o600)
		}
	}
	r := mustInit(t, opts(target, source))
	for _, l := range r.Lines {
		if l.Status != stack.StatusAdopted && l.Status != stack.StatusMissing && l.Status != stack.StatusPending {
			t.Errorf("restore reports %s %s, want adopted, missing or pending", l.Status, l.Name)
		}
	}
	_, restored := untar(t, exportBytes(t, filepath.Join(target, "_store"), id), false, id)
	compareEntries(t, restored, want)
}

// exportBytes exports dir to a buffer for id and returns the bundle.
func exportBytes(t *testing.T, dir string, id *age.X25519Identity) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := stack.Export(t.Context(), stack.ExportOptions{
		From: dir, Recipients: []string{id.Recipient().String()}, Stdout: &b, IsTerminal: notTerminal,
	}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// keyName is a key store file name of the form <purpose>-<kid>.pem.
const keyName = "invocation-EXAMPLE_kid.pem"

func TestSecretsExport_IncludesKeyStore(t *testing.T) {
	t.Parallel()
	from, want := store(t)
	keys := filepath.Join(t.TempDir(), "signing-keys")
	if err := os.Mkdir(keys, 0o700); err != nil {
		t.Fatal(err)
	}
	put(t, keys, keyName, "EXAMPLE-private-key-pem", 0o600)
	put(t, keys, "access_token-EXAMPLE_kid2.pem", "EXAMPLE-second-key", 0o600)
	put(t, keys, ".bothy-tmp-leftover", "EXAMPLE-partial", 0o600)              // never in the bundle
	if err := os.Mkdir(filepath.Join(keys, "lost+found"), 0o700); err != nil { // an ext4 mount; never in the bundle
		t.Fatal(err)
	}
	id := identity(t)
	var out bytes.Buffer
	err := stack.Export(t.Context(), stack.ExportOptions{
		From: from, SigningKeyDir: keys, Recipients: []string{id.Recipient().String()},
		Stdout: &out, IsTerminal: notTerminal,
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte("EXAMPLE-private-key-pem")) {
		t.Fatal("bundle contains a private key in plaintext")
	}
	want["signing-keys/"] = tarEntry{dir: true, mode: 0o700}
	want["signing-keys/"+keyName] = tarEntry{mode: 0o600, content: "EXAMPLE-private-key-pem"}
	want["signing-keys/access_token-EXAMPLE_kid2.pem"] = tarEntry{mode: 0o600, content: "EXAMPLE-second-key"}
	names, got := untar(t, out.Bytes(), false, id)
	compareEntries(t, got, want)
	if !slices.IsSorted(names) {
		t.Errorf("bundle names are not sorted: %v", names)
	}
}

func TestSecretsExport_KeyStoreRefusesUnknownEntries(t *testing.T) {
	t.Parallel()
	from, _ := store(t)
	for name, prepare := range map[string]func(t *testing.T, keys string){
		"unknown file": func(t *testing.T, keys string) { put(t, keys, "notes.txt", "EXAMPLE", 0o600) },
		"directory": func(t *testing.T, keys string) {
			if err := os.Mkdir(filepath.Join(keys, "sub"), 0o700); err != nil {
				t.Fatal(err)
			}
		},
		"link": func(t *testing.T, keys string) {
			if err := os.Symlink("/etc/hosts", filepath.Join(keys, keyName)); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			keys := t.TempDir()
			prepare(t, keys)
			var out bytes.Buffer
			err := stack.Export(t.Context(), stack.ExportOptions{
				From: from, SigningKeyDir: keys, Recipients: []string{identity(t).Recipient().String()},
				Stdout: &out, IsTerminal: notTerminal,
			})
			if err == nil {
				t.Fatal("Export accepted a key store with an unknown entry")
			}
			if out.Len() != 0 {
				t.Errorf("Export wrote %d bytes before failing", out.Len())
			}
		})
	}
}

func TestSecretsExport_MissingKeyStore(t *testing.T) {
	t.Parallel()
	from, want := store(t)
	missing := filepath.Join(t.TempDir(), "missing")
	id := identity(t)
	var out bytes.Buffer
	opts := stack.ExportOptions{
		From: from, SigningKeyDir: missing, Recipients: []string{id.Recipient().String()},
		Stdout: &out, IsTerminal: notTerminal,
	}
	if err := stack.Export(t.Context(), opts); !errors.Is(err, stack.ErrUsage) {
		t.Fatalf("Export with a missing key store = %v, want a usage error", err)
	}
	var note bytes.Buffer
	opts.SigningKeyDirOptional, opts.Notice = true, &note
	if err := stack.Export(t.Context(), opts); err != nil {
		t.Fatalf("Export with an optional missing key store: %v", err)
	}
	if !strings.Contains(note.String(), "signing key store") {
		t.Errorf("notice %q does not mention the skipped key store", note.String())
	}
	_, got := untar(t, out.Bytes(), false, id)
	compareEntries(t, got, want)
}
