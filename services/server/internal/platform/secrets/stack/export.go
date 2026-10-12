// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"archive/tar"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// ExportOptions are the flags of bothy secrets export.
type ExportOptions struct {
	// From is the _store directory whose tree becomes the bundle.
	From string
	// Recipients are X25519 age public keys ("age1..."); every one can decrypt the bundle.
	Recipients []string
	// Output is a file to create with mode 0600, or "" or "-" for Stdout.
	Output string
	// Armor writes the PEM-like ASCII armor of age instead of binary.
	Armor bool
	// Stdout receives the bundle when Output is "" or "-".
	Stdout io.Writer
	// IsTerminal reports whether a writer is a terminal; nil checks for a character device.
	IsTerminal func(io.Writer) bool
	// SigningKeyDir is the key store of the signing keys (secrets.signing_key_dir, ADR 0066 rule
	// 5); its files go into the bundle under KeyStorePrefix. Empty exports _store only.
	SigningKeyDir string
	// SigningKeyDirOptional lets a SigningKeyDir that does not exist be skipped with a line on
	// Notice instead of failing the export (the default path of the command on a host without
	// a key store).
	SigningKeyDirOptional bool
	// Notice receives that line; nil discards it.
	Notice io.Writer
}

// KeyStorePrefix is the directory of the bundle that holds the files of the key store
// secrets.signing_key_dir; everything else in the bundle is the _store tree.
const KeyStorePrefix = "signing-keys/"

// keyFilePattern is the name of a key store file: <purpose>-<kid>.pem with the purpose of 03
// server_signing_keys and a kid of base64url characters (RFC 7638 thumbprint, ADR 0066 rule 3).
var keyFilePattern = regexp.MustCompile(`^[a-z][a-z_]*-[A-Za-z0-9_-]+\.pem$`)

// bundleEntry is one tar member: its name in the bundle and the scan result of its source.
type bundleEntry struct {
	name string // relative slash path, directories with a trailing slash
	path string // path inside root
	info fs.FileInfo
	root *os.Root // the tree the entry is read from: From, or the key store
}

// Export writes the escrow bundle (docs/spec/16-betrieb.md 5.4, brief D8): a POSIX tar stream
// (PAX) of From with relative names, permission bits, uid and gid 0 without names and mtime 0, in
// sorted order, encrypted with age to every recipient and optionally armored. With
// opts.SigningKeyDir the files of the key store follow under KeyStorePrefix (keyStoreEntries). Plaintext never
// touches a disk. Recipients and the output target are checked before anything is written; an
// output file is created exclusively and removed again when the export fails. Errors of the
// options wrap ErrUsage.
func Export(ctx context.Context, opts ExportOptions) error {
	return escaped(export(ctx, opts))
}

// export checks recipients, output and source in this order before anything is written, then
// writes the bundle to standard output or to a new file.
func export(ctx context.Context, opts ExportOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	recipients, err := parseRecipients(opts.Recipients)
	if err != nil {
		return err
	}
	toStdout, err := checkOutput(opts)
	if err != nil {
		return err
	}
	if opts.From == "" {
		return usagef("the _store directory to export is required")
	}
	if err := requireDir(opts.From, "export source"); err != nil {
		return err
	}
	root, err := os.OpenRoot(opts.From)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	entries, err := bundleEntries(root)
	if err != nil {
		return err
	}
	keyRoot, keyEntries, err := keyStoreEntries(opts)
	if err != nil {
		return err
	}
	if keyRoot != nil {
		defer func() { _ = keyRoot.Close() }()
		entries = append(entries, keyEntries...)
		slices.SortFunc(entries, func(a, b bundleEntry) int { return cmp.Compare(a.name, b.name) })
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if toStdout {
		return writeBundle(ctx, opts.Stdout, opts.Armor, recipients, entries)
	}
	return writeBundleFile(ctx, opts.Output, opts.Armor, recipients, entries)
}

// keyStoreEntries lists the key store of opts.SigningKeyDir for the bundle: the directory as
// KeyStorePrefix and every regular file named <purpose>-<kid>.pem below it. Leftover temp files of
// an interrupted Put are skipped; any other entry (a subdirectory, a link, an unknown name) stops
// the export, because a restore must not install a file the server never wrote. A missing
// directory is a usage error, or a line on opts.Notice when SigningKeyDirOptional. The returned
// root is nil when there is nothing to add.
func keyStoreEntries(opts ExportOptions) (*os.Root, []bundleEntry, error) {
	if opts.SigningKeyDir == "" {
		return nil, nil, nil
	}
	info, err := os.Stat(opts.SigningKeyDir)
	if errors.Is(err, fs.ErrNotExist) && opts.SigningKeyDirOptional {
		if opts.Notice != nil {
			_, _ = fmt.Fprintf(opts.Notice, "signing key store %s not found; the bundle holds _store only\n", display(opts.SigningKeyDir))
		}
		return nil, nil, nil
	}
	if err := requireDir(opts.SigningKeyDir, "signing key store"); err != nil {
		return nil, nil, err
	}
	root, err := os.OpenRoot(opts.SigningKeyDir)
	if err != nil {
		return nil, nil, err
	}
	out := []bundleEntry{{name: KeyStorePrefix, path: ".", info: info, root: root}}
	des, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		_ = root.Close()
		return nil, nil, err
	}
	for _, de := range des {
		e, skip, err := keyStoreEntry(root, de.Name())
		if err != nil {
			_ = root.Close()
			return nil, nil, err
		}
		if !skip {
			out = append(out, e)
		}
	}
	return root, out, nil
}

// keyStoreEntry returns the bundle entry of the key store file name: skip for a leftover temp
// file and for the directory lost+found, an error for anything but a regular file named
// <purpose>-<kid>.pem.
func keyStoreEntry(root *os.Root, name string) (bundleEntry, bool, error) {
	if isTemp(name) {
		return bundleEntry{}, true, nil
	}
	fi, err := root.Lstat(name)
	if err != nil {
		return bundleEntry{}, false, err
	}
	if name == "lost+found" && fi.IsDir() {
		// The root of an ext4 file system mounted as the key store.
		return bundleEntry{}, true, nil
	}
	if !fi.Mode().IsRegular() || !keyFilePattern.MatchString(name) {
		return bundleEntry{}, false, usagef("signing key store holds %s, which is not a key file <purpose>-<kid>.pem", display(name))
	}
	return bundleEntry{name: KeyStorePrefix + name, path: name, info: fi, root: root}, false, nil
}

// checkOutput reports whether the bundle goes to standard output ("" or "-") and refuses that when
// there is no standard output, or when it is a terminal and the bundle is binary: binary output
// would garble the terminal and leave no usable bundle.
func checkOutput(opts ExportOptions) (toStdout bool, err error) {
	if opts.Output != "" && opts.Output != "-" {
		return false, nil
	}
	if opts.Stdout == nil {
		return true, errors.New("no standard output to write the bundle to")
	}
	isTerminal := opts.IsTerminal
	if isTerminal == nil {
		isTerminal = charDevice
	}
	if !opts.Armor && isTerminal(opts.Stdout) {
		return true, usagef("refusing to write the binary bundle to a terminal: redirect the output, name a file or add --armor")
	}
	return true, nil
}

// parseRecipients accepts only X25519 recipients. A rejected recipient is named by its position:
// an identity pasted by mistake ("AGE-SECRET-KEY-1...") must not reach the terminal or a log, and
// the age parser quotes its input in its errors.
func parseRecipients(raw []string) ([]age.Recipient, error) {
	if len(raw) == 0 {
		return nil, usagef("at least one age recipient (age1...) is required")
	}
	out := make([]age.Recipient, 0, len(raw))
	for i, s := range raw {
		r, err := age.ParseX25519Recipient(s)
		if err != nil {
			return nil, usagef("age recipient %d of %d is not an X25519 public key (age1...)", i+1, len(raw))
		}
		out = append(out, r)
	}
	return out, nil
}

// charDevice is the default terminal check: a terminal is a character device.
func charDevice(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&fs.ModeCharDevice != 0
}

// bundleEntries lists From in bundle order. Leftover temp files of an interrupted init are
// skipped; links, devices and pipes stop the export instead of being archived or followed.
func bundleEntries(root *os.Root) ([]bundleEntry, error) {
	found, err := scan(root, ".")
	if err != nil {
		return nil, err
	}
	var out []bundleEntry
	var unknown []string
	for _, rel := range slices.Sorted(maps.Keys(found)) {
		info := found[rel]
		switch {
		case info.IsDir():
			out = append(out, bundleEntry{name: rel + "/", path: rel, info: info, root: root})
		case isTemp(rel):
		case !info.Mode().IsRegular():
			return nil, fmt.Errorf("%s is not a regular file or directory", display(rel))
		case classify(rel) < 0:
			unknown = append(unknown, display(rel))
		default:
			out = append(out, bundleEntry{name: rel, path: rel, info: info, root: root})
		}
	}
	if len(unknown) > 0 {
		// The bundle is what init adopts as --source (disaster-recovery.md Fall B), and init
		// refuses files the catalog does not name; a bundle with them would fail at the restore.
		return nil, usagef("export source holds files without a catalog entry (docs/operations/configuration.md section 5): %s; --from must be the _store directory",
			strings.Join(unknown, ", "))
	}
	// "a/" sorts after "a-b" although "a" sorts before it.
	slices.SortFunc(out, func(a, b bundleEntry) int { return cmp.Compare(a.name, b.name) })
	return out, nil
}

// writeBundleFile creates the output exclusively with mode 0600, so an existing bundle is never
// overwritten, and removes it again when anything fails.
func writeBundleFile(ctx context.Context, output string, armored bool, recipients []age.Recipient, entries []bundleEntry) (err error) {
	name := filepath.Clean(output)
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	// The umask may have cleared bits; the mode is set exactly.
	if err := f.Chmod(0o600); err != nil {
		return err
	}
	if err := writeBundle(ctx, f, armored, recipients, entries); err != nil {
		return err
	}
	return f.Sync()
}

// writeBundle streams tar into age into the optional armor into w.
func writeBundle(ctx context.Context, w io.Writer, armored bool, recipients []age.Recipient, entries []bundleEntry) error {
	dst := w
	var armorWriter io.WriteCloser
	if armored {
		armorWriter = armor.NewWriter(w)
		dst = armorWriter
	}
	enc, err := age.Encrypt(dst, recipients...)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(enc)
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeEntry(tw, e); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if armorWriter != nil {
		return armorWriter.Close()
	}
	return nil
}

// writeEntry writes the tar header and, for a file, the content of one bundle entry. The header
// holds only name, permission bits, type and size: uid and gid stay 0 without names and the mtime
// is 0 (brief D8), so the tar stream depends on the tree of From alone, not on the host.
func writeEntry(tw *tar.Writer, e bundleEntry) error {
	hdr := &tar.Header{
		Name:    e.name,
		Mode:    int64(e.info.Mode().Perm()),
		ModTime: time.Unix(0, 0),
		Format:  tar.FormatPAX,
	}
	var data []byte
	if e.info.IsDir() {
		hdr.Typeflag = tar.TypeDir
	} else {
		var err error
		if data, err = readRegular(e.root, e.path, e.info); err != nil {
			return fmt.Errorf("%s: %w", display(e.path), err)
		}
		hdr.Typeflag = tar.TypeReg
		hdr.Size = int64(len(data))
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
