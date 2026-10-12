// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	// ErrUsage marks errors in the options of init or export (exit code 2): unknown source file,
	// non-regular source entry, invalid or missing owner, missing recipient.
	ErrUsage = errors.New("usage error")
	// ErrConflict marks a generated secret whose source copy differs from _store without
	// AdoptSource (exit code 1, like every other error that does not wrap ErrUsage).
	ErrConflict = errors.New("conflict")
)

// classError carries its own message and wraps the class sentinel, so callers decide the exit code
// with errors.Is while the message reads without a "usage error:" prefix.
type classError struct {
	class error
	msg   string
}

func (e *classError) Error() string { return e.msg }
func (e *classError) Unwrap() error { return e.class }

func usagef(format string, args ...any) error {
	return &classError{class: ErrUsage, msg: fmt.Sprintf(format, args...)}
}

// Status is the outcome of init for one entry, the first word of its report line.
type Status string

const (
	// StatusGenerated means the entry was absent and init created it.
	StatusGenerated Status = "generated"
	// StatusAdopted means the entry was absent in _store and init copied it from the source.
	StatusAdopted Status = "adopted"
	// StatusKept means _store already held the entry and keeps it.
	StatusKept Status = "kept"
	// StatusUpdated means the source differed and replaced the entry (supplied, or AdoptSource).
	StatusUpdated Status = "updated"
	// StatusMissing means the owner has not supplied the entry; services that need it fail.
	StatusMissing Status = "missing"
	// StatusPending means the entry is absent and its generator comes with a later ticket
	// (Entry.PendingOn).
	StatusPending Status = "pending"
)

// Line is one line of the report. It never holds a value, only the fingerprint: "sha256:" and the
// first 12 hex characters of the SHA-256 of the content, enough to compare two hosts. Like any
// unsalted hash it still confirms a guessed low-entropy value (a weak supplied password, a webhook
// URL), so the report, which the log of the secrets-init container keeps, is not for public places.
type Line struct {
	Status      Status
	Name        string
	Fingerprint string
}

// String renders "<status> <name> <fingerprint>", or "<status> <name>" without a fingerprint.
func (l Line) String() string {
	if l.Fingerprint == "" {
		return string(l.Status) + " " + l.Name
	}
	return string(l.Status) + " " + l.Name + " " + l.Fingerprint
}

// Report lists the outcome of init in catalog order; pattern entries give one line per file.
type Report struct {
	// Store is the path of the _store directory the run prepared; Reminder names it.
	Store string
	Lines []Line
}

// shellWord matches a path that a shell reads as one word without quotes.
var shellWord = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

// Reminder returns the last line of every report (docs/spec/16-betrieb.md 5.3 item 5): without an
// escrow bundle a lost host also loses every backup. It is the export command to copy, with
// --from set to Store, because export needs it (brief D8, docs/DECISIONS.md M1-09/23); a path that
// is no plain shell word is single-quoted, an unprintable character escaped, so the line stays one
// line. Without Store it names the place to fill in.
func (r Report) Reminder() string {
	from := "<volume>/_store"
	if r.Store != "" {
		from = escape(r.Store)
		if !shellWord.MatchString(from) {
			from = "'" + strings.ReplaceAll(from, "'", `'\''`) + "'"
		}
	}
	return "back up the secrets: bothy secrets export --from " + from + " --age-recipient <age public key>"
}

// WriteTo writes one line per entry and Reminder as the last line; it implements io.WriterTo.
func (r Report) WriteTo(w io.Writer) (int64, error) {
	var b strings.Builder
	for _, l := range r.Lines {
		b.WriteString(l.String())
		b.WriteByte('\n')
	}
	b.WriteString(r.Reminder())
	b.WriteByte('\n')
	n, err := io.WriteString(w, b.String())
	return int64(n), err
}

// Owner is the uid and gid a service's copies belong to.
type Owner struct {
	UID, GID int
}

// ownerSyntax is the form of --owner. The service has the form of a catalog service name, so a
// name that Init reports as unknown is never text of another kind (a pasted key).
var ownerSyntax = regexp.MustCompile(`^([a-z][a-z0-9-]*)=([0-9]+):([0-9]+)$`)

// ParseOwner parses the flag value "<service>=<uid>:<gid>": the service in lower-case letters,
// digits and "-", decimal ids from 0 to 2147483647 (the largest id every supported kernel
// accepts). It checks only the form; Init checks that the service exists. Errors wrap ErrUsage
// and never repeat s: a value given to the wrong flag may be a key (docs/DECISIONS.md M1-09/21).
func ParseOwner(s string) (service string, o Owner, err error) {
	m := ownerSyntax.FindStringSubmatch(s)
	if m == nil {
		return "", Owner{}, usagef("want <service>=<uid>:<gid>, the service in lower-case letters, digits and -")
	}
	uid, uerr := strconv.ParseInt(m[2], 10, 32)
	gid, gerr := strconv.ParseInt(m[3], 10, 32)
	if uerr != nil || gerr != nil {
		return "", Owner{}, usagef("uid and gid must be between 0 and %d", math.MaxInt32)
	}
	return m[1], Owner{UID: int(uid), GID: int(gid)}, nil
}

// InitOptions are the flags of bothy secrets init.
type InitOptions struct {
	// Target is the volume root: _store and one directory per service go below it.
	Target string
	// Source is the directory of supplied files (and of a decrypted escrow bundle); "" for none.
	Source string
	// Owners gives a service directory 0500/0400 copies owned by uid:gid; server and migrate need
	// one because the file driver reads only files of its own user.
	Owners map[string]Owner
	// AdoptSource lets a differing source copy replace a generated secret (restore from the
	// bundle, disaster-recovery.md "Fall B") instead of stopping with a conflict.
	AdoptSource bool
	// Rand is the entropy of the generators; nil means crypto/rand.Reader.
	Rand io.Reader
}

const (
	storeDir     = "_store"
	storeDirMode = 0o700
	secretMode   = 0o400 // files in _store and copies of services with an owner
	ownedDirMode = 0o500
	// Services without an owner run third-party images with their own users; isolation comes
	// from the subpath mount, so their copies are world-readable (brief D7).
	sharedDirMode  = 0o755
	sharedFileMode = 0o444
	// openDirMode lets the running user change a directory; its final mode is set afterwards.
	openDirMode = 0o700
)

// item is one line of the report: a file entry, a directory entry as a whole, or one file of a
// pattern entry. Paths in the maps are relative to _store (and to each service directory).
type item struct {
	entry  Entry
	name   string
	status Status
	stored map[string][]byte // _store content before the run
	source map[string][]byte // source content
	final  map[string][]byte // _store content after the run; nil for missing and pending
}

// tree is the scanned content of _store or the source: known files with their content, scan
// results for every entry, and leftover temp files.
type tree struct {
	content map[string][]byte
	info    map[string]fs.FileInfo
	temps   []string
}

// Init prepares the secret volume of the Compose stack (docs/spec/16-betrieb.md 5.3, brief D4 to
// D7): it reconciles _store with the source, generates what is absent, and makes every service
// directory hold exactly the copies its catalog entries name. Every check (options, source
// files, owners, conflicts, entropy) runs before the first write; a second run with the same
// inputs writes nothing. A run holds the target for its whole duration, so a concurrent run on
// the same target stops with an error instead of interleaving. On error the Report is empty.
func Init(ctx context.Context, opts InitOptions) (Report, error) {
	r, err := initStack(ctx, opts)
	return r, escaped(err)
}

// initStack runs the phases of Init in their fixed order: every check, the locked read of an
// existing target, the decisions and generated values, the creation of a missing target, and
// only then the writes to _store and the service directories.
func initStack(ctx context.Context, opts InitOptions) (Report, error) {
	in, err := checkInit(ctx, opts)
	if err != nil {
		return Report{}, err
	}
	var v volume
	defer v.close()
	exists, store, err := v.openExisting(in.target)
	if err != nil {
		return Report{}, err
	}
	items, err := plan(ctx, in.src, store, opts)
	if err != nil {
		return Report{}, err
	}
	if !exists {
		if err := v.create(in.target); err != nil {
			return Report{}, err
		}
	}
	if err := applyStore(v.root, items, store, in.run); err != nil {
		return Report{}, err
	}
	if err := applyServices(ctx, v.root, items, opts.Owners, in.def); err != nil {
		return Report{}, err
	}
	r := report(items)
	r.Store = filepath.Join(in.target, storeDir)
	return r, nil
}

// checked is what the checks before the first access to the target give: the clean target, the
// scanned source, the owner of _store and the owner of copies for services without an owner.
type checked struct {
	target   string
	src      tree
	run, def Owner
}

// checkInit runs every check that needs no access to the target in a fixed order, so the first
// failing check is the one reported: context, options, source, owners.
func checkInit(ctx context.Context, opts InitOptions) (checked, error) {
	if err := ctx.Err(); err != nil {
		return checked{}, err
	}
	target, err := checkInitOptions(opts)
	if err != nil {
		return checked{}, err
	}
	src, err := readSource(opts.Source)
	if err != nil {
		return checked{}, err
	}
	run, def, err := checkOwners(opts.Owners)
	if err != nil {
		return checked{}, err
	}
	return checked{target: target, src: src, run: run, def: def}, nil
}

// volume is the open target of a run and the lock the run holds on it. A run takes the lock once,
// either on the existing target before _store is read or on the new target after it is created.
type volume struct {
	root    *os.Root
	release func()
}

// close releases the lock before it closes the root, the reverse order of taking them.
func (v *volume) close() {
	if v.release != nil {
		v.release()
	}
	if v.root != nil {
		_ = v.root.Close()
	}
}

// openExisting opens and locks the target when it exists and then reads _store; the lock comes
// first, so no other run changes _store while it is read. A missing target leaves v empty and
// gives an empty store.
func (v *volume) openExisting(target string) (exists bool, store tree, err error) {
	store = tree{content: map[string][]byte{}, info: map[string]fs.FileInfo{}}
	root, exists, err := openTarget(target)
	if err != nil || !exists {
		return exists, store, err
	}
	v.root = root
	release, err := lock(root, target)
	if err != nil {
		return exists, store, err
	}
	v.release = release
	if err := checkVolume(root, target); err != nil {
		return exists, store, err
	}
	store, err = readStore(root)
	return exists, store, err
}

// checkVolume refuses an existing target that holds entries but no _store: init prepares only an
// empty directory or an existing secrets volume. A mistyped --target (/var/lib) would otherwise be
// taken over by the running user and every subdirectory named like a service emptied
// (docs/DECISIONS.md M1-09/29). lost+found, which a dedicated file system brings, counts as empty.
func checkVolume(root *os.Root, target string) error {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == storeDir {
			return nil
		}
	}
	for _, e := range entries {
		if e.Name() != "lost+found" {
			return usagef("target %s holds files but no %s; init prepares only an empty directory or an existing secrets volume", display(target), storeDir)
		}
	}
	return nil
}

// create makes the missing target only after every check passed and locks it. Another run may
// have opened the new target and finished before this one took the lock; the items were decided
// for an empty target, so they would replace its secrets, and the run stops instead.
func (v *volume) create(target string) error {
	root, err := createTarget(target)
	if errors.Is(err, fs.ErrExist) {
		return busy(target)
	}
	if err != nil {
		return err
	}
	v.root = root
	release, err := lock(root, target)
	if err != nil {
		return err
	}
	v.release = release
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return busy(target)
	}
	return nil
}

// plan decides every item and generates the absent values before anything is written, so a
// conflict or failing entropy leaves no half-prepared volume.
func plan(ctx context.Context, src, store tree, opts InitOptions) ([]*item, error) {
	items, err := decide(src, store, opts.AdoptSource)
	if err != nil {
		return nil, err
	}
	rnd := opts.Rand
	if rnd == nil {
		rnd = rand.Reader
	}
	if err := generate(items, rnd); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// applyServices gives every service of the catalog its directory, even with nothing to copy,
// because a Compose subpath mount of a missing directory fails.
func applyServices(ctx context.Context, root *os.Root, items []*item, owners map[string]Owner, def Owner) error {
	for _, service := range Services() {
		if err := ctx.Err(); err != nil {
			return err
		}
		dirMode, fileMode, o := fs.FileMode(sharedDirMode), fs.FileMode(sharedFileMode), def
		if owner, ok := owners[service]; ok {
			dirMode, fileMode, o = ownedDirMode, secretMode, owner
		}
		want := map[string][]byte{}
		for _, it := range items {
			if it.final != nil && slices.Contains(it.entry.Services, service) {
				maps.Copy(want, it.final)
			}
		}
		if err := syncService(root, service, want, dirMode, fileMode, o); err != nil {
			return fmt.Errorf("service directory %s: %w", service, err)
		}
	}
	return nil
}

// checkInitOptions validates the options before anything is read: target, owners and that the
// source is a directory.
func checkInitOptions(opts InitOptions) (string, error) {
	if opts.Target == "" {
		return "", usagef("a target directory is required")
	}
	services := Services()
	var unknown []string
	for _, s := range slices.Sorted(maps.Keys(opts.Owners)) {
		if !slices.Contains(services, s) {
			unknown = append(unknown, display(s))
			continue
		}
		o := opts.Owners[s]
		if o.UID < 0 || o.UID > math.MaxInt32 || o.GID < 0 || o.GID > math.MaxInt32 {
			return "", usagef("owner of %s: uid and gid must be between 0 and %d", s, math.MaxInt32)
		}
	}
	if len(unknown) > 0 {
		return "", usagef("owner for unknown service %s; services are %s", strings.Join(unknown, ", "), strings.Join(services, ", "))
	}
	for _, s := range []string{serviceServer, serviceMigrate} {
		if _, ok := opts.Owners[s]; !ok {
			return "", usagef("service %s needs an owner <uid>:<gid>: the server reads only secret files of its own user", s)
		}
	}
	if opts.Source != "" {
		if err := requireDir(opts.Source, "source"); err != nil {
			return "", err
		}
	}
	return filepath.Clean(opts.Target), nil
}

// readSource scans and reads the source directory. Every entry must be a directory or a regular
// file the catalog names: init runs as root over a directory the owner fills, so a link is never
// followed and a typo never silently skipped. The one exception is the key store part of a
// bundle under KeyStorePrefix, which init skips.
func readSource(dir string) (tree, error) {
	src := tree{content: map[string][]byte{}, info: map[string]fs.FileInfo{}}
	if dir == "" {
		return src, nil
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return src, err
	}
	defer func() { _ = root.Close() }()
	if src.info, err = scan(root, "."); err != nil {
		return src, err
	}
	// A decrypted bundle carries the key store under KeyStorePrefix; it belongs to the volume of
	// secrets.signing_key_dir, not to _store, so init leaves it alone (M1-32).
	maps.DeleteFunc(src.info, func(rel string, _ fs.FileInfo) bool {
		return rel+"/" == KeyStorePrefix || strings.HasPrefix(rel, KeyStorePrefix)
	})
	if err := checkSourceEntries(src.info); err != nil {
		return src, err
	}
	for rel, info := range src.info {
		if info.Mode().IsRegular() {
			if src.content[rel], err = readRegular(root, rel, info); err != nil {
				return src, fmt.Errorf("source %s: %w", display(rel), err)
			}
		}
	}
	if empty := emptyValues(src.content); len(empty) > 0 {
		return src, usagef("source files without a value (empty or only a line end): %s", strings.Join(empty, ", "))
	}
	return src, nil
}

// emptyValues returns the sorted names of the files of file and pattern entries whose content is
// empty after the line end rule of the file driver (one trailing "\n" and a "\r" before it): an
// empty password or key is never valid (docs/DECISIONS.md M1-09/2), and the services that read
// these files directly would get it. Files below a directory entry (offsite/) may be empty.
func emptyValues(content map[string][]byte) []string {
	var empty []string
	for _, rel := range slices.Sorted(maps.Keys(content)) {
		i := classify(rel)
		if i < 0 || catalog[i].isDir() {
			continue
		}
		v := bytes.TrimSuffix(content[rel], []byte("\n"))
		if len(bytes.TrimSuffix(v, []byte("\r"))) == 0 {
			empty = append(empty, display(rel))
		}
	}
	return empty
}

// checkSourceEntries refuses a source with an entry that is not a directory or regular file, or
// with a file the catalog does not name. It names every such entry at once, before any file is
// read, so the owner can fix the source in one go.
func checkSourceEntries(found map[string]fs.FileInfo) error {
	var irregular, unknown []string
	for _, rel := range slices.Sorted(maps.Keys(found)) {
		info := found[rel]
		switch {
		case info.IsDir():
		case !info.Mode().IsRegular():
			irregular = append(irregular, display(rel))
		case isTemp(rel) || classify(rel) < 0:
			unknown = append(unknown, display(rel))
		}
	}
	var problems []string
	if len(irregular) > 0 {
		problems = append(problems, "source entries that are not a regular file (links, devices and pipes are refused): "+strings.Join(irregular, ", "))
	}
	if len(unknown) > 0 {
		problems = append(problems, "source files without a catalog entry (docs/operations/configuration.md section 5): "+strings.Join(unknown, ", "))
	}
	if len(problems) > 0 {
		return usagef("%s", strings.Join(problems, "; "))
	}
	return nil
}

// checkOwners returns the running user (owner of _store) and the owner of copies for services
// without an owner: root when running as root, else the running user, who can then give no
// other owner (brief D7).
func checkOwners(owners map[string]Owner) (run, def Owner, err error) {
	if run, err = runningUser(); err != nil {
		return Owner{}, Owner{}, err
	}
	if run.UID == 0 {
		return run, Owner{}, nil
	}
	var foreign []string
	for _, s := range slices.Sorted(maps.Keys(owners)) {
		if owners[s] != run {
			foreign = append(foreign, fmt.Sprintf("%s=%d:%d", s, owners[s].UID, owners[s].GID))
		}
	}
	if len(foreign) > 0 {
		return Owner{}, Owner{}, fmt.Errorf("not running as root (uid %d, gid %d): cannot give copies another owner: %s",
			run.UID, run.GID, strings.Join(foreign, ", "))
	}
	return run, run, nil
}

// openTarget opens the target when it exists. A missing target is created only after every
// check passed, so a run that fails its checks leaves no trace.
func openTarget(target string) (root *os.Root, exists bool, err error) {
	info, err := os.Stat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.IsDir() {
		return nil, false, fmt.Errorf("target %s is not a directory", display(target))
	}
	root, err = os.OpenRoot(target)
	return root, err == nil, err
}

// errTargetLocked is what lockTarget returns when another run holds the target.
var errTargetLocked = errors.New("target is locked")

// lock holds the target for the rest of the run, from before _store is read until the last write:
// a second run would otherwise remove the temp files of the first as leftovers and could generate
// a value over a secret the first has just written and copied (brief requirement 15).
func lock(root *os.Root, target string) (release func(), err error) {
	release, err = lockTarget(root)
	if errors.Is(err, errTargetLocked) {
		return nil, busy(target)
	}
	return release, err
}

// busy is the runtime error (exit code 1) of a run that found another run on its target.
func busy(target string) error {
	return fmt.Errorf("another secrets init is running on %s", display(target))
}

// createTarget creates the volume root with mode 0700: the service directories below it are
// world-readable for third-party images, so the root is what keeps other host users out.
func createTarget(target string) (*os.Root, error) {
	if err := os.Mkdir(target, storeDirMode); err != nil {
		return nil, err
	}
	return os.OpenRoot(target)
}

// readStore scans _store. Files the catalog does not name are left alone; temp files are
// leftovers to remove; anything but directories and regular files means the store was tampered
// with, and init stops rather than copy it.
func readStore(root *os.Root) (tree, error) {
	st := tree{content: map[string][]byte{}}
	var err error
	if st.info, err = scan(root, storeDir); err != nil {
		return st, err
	}
	for _, rel := range slices.Sorted(maps.Keys(st.info)) {
		info := st.info[rel]
		switch {
		case info.IsDir():
		case isTemp(rel):
			st.temps = append(st.temps, rel)
		case !info.Mode().IsRegular():
			return st, fmt.Errorf("%s/%s is not a regular file", storeDir, display(rel))
		case classify(rel) >= 0:
			if st.content[rel], err = readRegular(root, storeDir+"/"+rel, info); err != nil {
				return st, fmt.Errorf("%s/%s: %w", storeDir, display(rel), err)
			}
		}
	}
	if empty := emptyValues(st.content); len(empty) > 0 {
		// Only a change outside init leaves an empty value in _store. Keeping it would copy it
		// to the services forever; replacing it could hide that change, so the run stops.
		return st, fmt.Errorf("%s holds no value (empty or only a line end) for %s; remove the file and run init again, or supply the value with --source and --adopt-source",
			storeDir, strings.Join(empty, ", "))
	}
	return st, nil
}

// decide builds the items in catalog order and applies the table of brief D4. Conflicts are
// collected so one run names all of them.
func decide(src, store tree, adopt bool) ([]*item, error) {
	var items []*item
	for i, e := range catalog {
		items = append(items, entryItems(e, filesOf(store.content, i), filesOf(src.content, i))...)
	}
	var conflicts []string
	for _, it := range items {
		if !it.settle(adopt) {
			conflicts = append(conflicts, it.name)
		}
	}
	if len(conflicts) > 0 {
		return nil, &classError{class: ErrConflict, msg: fmt.Sprintf(
			"the source holds a different value for the generated secrets %s; _store keeps them unless --adopt-source replaces them",
			strings.Join(conflicts, ", "))}
	}
	return items, nil
}

// entryItems returns the items of one catalog entry from its stored and supplied files: one item
// for a file or directory entry; for a pattern entry one per file present on either side, sorted
// by name, and the placeholder name when there is none, so the report always names the entry.
func entryItems(e Entry, stored, supplied map[string][]byte) []*item {
	if e.Pattern == "" {
		return []*item{{entry: e, name: e.Name, stored: stored, source: supplied}}
	}
	names := slices.Sorted(maps.Keys(stored))
	for rel := range supplied {
		if _, ok := stored[rel]; !ok {
			names = append(names, rel)
		}
	}
	slices.Sort(names)
	if len(names) == 0 {
		return []*item{{entry: e, name: e.Name, stored: stored, source: supplied}}
	}
	items := make([]*item, 0, len(names))
	for _, rel := range names {
		items = append(items, &item{entry: e, name: rel, stored: only(stored, rel), source: only(supplied, rel)})
	}
	return items
}

// settle sets status and final content of the item by the table of brief D4 and reports false
// for a conflict: a generated secret whose source copy differs from _store without adopt. A
// conflict leaves the item unset; decide collects all of them before it stops.
func (it *item) settle(adopt bool) bool {
	inStore, inSource := len(it.stored) > 0, len(it.source) > 0
	switch {
	case !inStore && !inSource:
		it.status = absentStatus(it.entry)
	case !inStore:
		it.status, it.final = StatusAdopted, it.source
	case !inSource || maps.EqualFunc(it.stored, it.source, bytes.Equal):
		it.status, it.final = StatusKept, it.stored
	case it.entry.Origin == OriginGenerated && !adopt:
		return false
	default:
		it.status, it.final = StatusUpdated, it.source
	}
	return true
}

// absentStatus is the status of an entry that neither _store nor the source holds: pending while
// its generator is missing, generated when init creates it (generate fills final), else missing.
func absentStatus(e Entry) Status {
	switch {
	case e.Generator == GeneratorPending:
		return StatusPending
	case e.Origin == OriginGenerated:
		return StatusGenerated
	default:
		return StatusMissing
	}
}

// filesOf returns the files that belong to catalog entry i.
func filesOf(files map[string][]byte, i int) map[string][]byte {
	out := map[string][]byte{}
	for rel, c := range files {
		if classify(rel) == i {
			out[rel] = c
		}
	}
	return out
}

// only returns the one file rel of files, or an empty map.
func only(files map[string][]byte, rel string) map[string][]byte {
	out := map[string][]byte{}
	if c, ok := files[rel]; ok {
		out[rel] = c
	}
	return out
}

// generate fills every item with status generated before anything is written, so failing
// entropy leaves no half-prepared volume (brief D5: 32 random bytes).
func generate(items []*item, rnd io.Reader) error {
	for _, it := range items {
		if it.status != StatusGenerated {
			continue
		}
		raw := make([]byte, 32)
		if _, err := io.ReadFull(rnd, raw); err != nil {
			return fmt.Errorf("generate %s: %w", it.name, err)
		}
		var value string
		switch it.entry.Generator {
		case GeneratorToken:
			value = base64.RawURLEncoding.EncodeToString(raw)
		case GeneratorKey:
			value = hex.EncodeToString(raw)
		case GeneratorNone, GeneratorPending:
			return fmt.Errorf("generate %s: entry has no generator", it.name)
		}
		it.final = map[string][]byte{it.name: []byte(value)}
	}
	return nil
}

// applyStore makes _store hold the final content of every item, repairs mode and owner of what
// it keeps and removes leftover temp files. Files the catalog does not name stay untouched.
func applyStore(root *os.Root, items []*item, st tree, run Owner) error {
	if err := repairRootMode(root, run); err != nil {
		return err
	}
	if err := ensureDir(root, storeDir, storeDirMode, run); err != nil {
		return err
	}
	// Directories that exist are repaired before anything is removed or written, so a drifted
	// mode cannot block either; the others are created after clearTree, which first removes a
	// stored file in their place.
	existingDirs, newDirs := storeDirs(items, st)
	if err := ensureStoreDirs(root, existingDirs, run); err != nil {
		return err
	}
	if err := removeTemps(root, st.temps); err != nil {
		return err
	}
	for _, it := range items {
		if err := clearTree(root, it, st); err != nil {
			return err
		}
	}
	if err := ensureStoreDirs(root, newDirs, run); err != nil {
		return err
	}
	return writeStore(root, items, st, run)
}

// repairRootMode gives the volume root the running user as owner and mode 0700 when either
// drifted: the service directories below it are world-readable, so the root is what keeps other
// host users out, and an owner other than the running user could open it again at any time.
func repairRootMode(root *os.Root, run Owner) error {
	info, err := root.Lstat(".")
	if err != nil {
		return err
	}
	if o, ok := ownerOf(info); !ok || o != run {
		if err := root.Lchown(".", run.UID, run.GID); err != nil {
			return fmt.Errorf("give the volume root to %d:%d: %w", run.UID, run.GID, err)
		}
	}
	if info.Mode().Perm() == storeDirMode {
		return nil
	}
	return root.Chmod(".", storeDirMode)
}

// storeDirs returns the directories of _store on the way to every known file, leftover and final
// file, sorted and split into those the scan found as directories and the others, which are
// missing or have a file in their place.
func storeDirs(items []*item, st tree) (existingDirs, newDirs []string) {
	dirs := map[string]bool{}
	for rel := range st.content {
		addParents(dirs, rel)
	}
	for _, rel := range st.temps {
		addParents(dirs, rel)
	}
	for _, it := range items {
		for rel := range it.final {
			addParents(dirs, rel)
		}
	}
	for _, d := range slices.Sorted(maps.Keys(dirs)) {
		if st.isDir(d) {
			existingDirs = append(existingDirs, d)
		} else {
			newDirs = append(newDirs, d)
		}
	}
	return existingDirs, newDirs
}

// addParents marks every directory on the way to the relative slash path rel, "." excluded.
func addParents(dirs map[string]bool, rel string) {
	for d := path.Dir(rel); d != "."; d = path.Dir(d) {
		dirs[d] = true
	}
}

// isDir reports whether the scan found rel as a directory.
func (t tree) isDir(rel string) bool {
	info, ok := t.info[rel]
	return ok && info.IsDir()
}

// ensureStoreDirs creates or repairs the directories dirs of _store in their order.
func ensureStoreDirs(root *os.Root, dirs []string, run Owner) error {
	for _, d := range dirs {
		if err := ensureDir(root, storeDir+"/"+d, storeDirMode, run); err != nil {
			return err
		}
	}
	return nil
}

// removeTemps removes the temp files an interrupted run left in _store.
func removeTemps(root *os.Root, temps []string) error {
	for _, rel := range temps {
		if err := root.Remove(storeDir + "/" + rel); err != nil {
			return err
		}
	}
	return nil
}

// writeStore writes the final files of every item that _store does not already hold with the
// same content, in item order and sorted within an item, and repairs mode and owner of the others.
func writeStore(root *os.Root, items []*item, st tree, run Owner) error {
	for _, it := range items {
		for _, rel := range slices.Sorted(maps.Keys(it.final)) {
			if err := storeFile(root, it, rel, st, run); err != nil {
				return err
			}
		}
	}
	return nil
}

// storeFile makes _store hold the final file rel of an item: an unchanged file only gets its mode
// and owner, so a second run changes no ctime; any other file is written atomically.
func storeFile(root *os.Root, it *item, rel string, st tree, run Owner) error {
	name := storeDir + "/" + rel
	if old, ok := it.stored[rel]; ok && bytes.Equal(old, it.final[rel]) {
		return setModeOwner(root, name, st.info[rel], secretMode, run)
	}
	if err := writeAtomic(root, path.Dir(name), path.Base(name), it.final[rel], secretMode, run); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// clearTree removes, before anything is written, what stands in the way of a directory item's
// final tree: stored files that left the source (among them a file where a directory goes now),
// stored directories where a file goes now, and the directories that became empty. Writing first
// would fail on every later run once a path below offsite/ changed between file and directory,
// and the D4 "replace" would never happen.
func clearTree(root *os.Root, it *item, st tree) error {
	if !it.entry.isDir() || it.final == nil {
		return nil
	}
	c := &clearing{
		root:    root,
		keep:    map[string]bool{strings.TrimSuffix(it.entry.Name, "/"): true},
		empty:   map[string]bool{},
		synced:  map[string]bool{},
		removed: map[string]bool{},
	}
	for rel := range it.final {
		addParents(c.keep, rel)
	}
	if err := c.removeBlocking(it, st); err != nil {
		return err
	}
	if err := c.removeLeft(it); err != nil {
		return err
	}
	if err := c.removeEmpty(); err != nil {
		return err
	}
	return c.sync()
}

// clearing is the state of clearTree for one directory item; paths are relative to _store.
type clearing struct {
	root     *os.Root
	keep     map[string]bool // the entry directory and the directories on the way to a final file
	blocking []string        // stored directories removed because a final file takes their place
	empty    map[string]bool // directories that may have become empty
	synced   map[string]bool // directories whose entries changed
	removed  map[string]bool // empty directories that were removed
}

// removeBlocking removes every stored directory in place of a final file with everything below
// it, also empty directories an interrupted run left behind.
func (c *clearing) removeBlocking(it *item, st tree) error {
	for _, rel := range slices.Sorted(maps.Keys(it.final)) {
		if !st.isDir(rel) {
			continue
		}
		if err := removeTree(c.root, storeDir+"/"+rel); err != nil {
			return err
		}
		c.blocking = append(c.blocking, rel)
		c.synced[path.Dir(rel)] = true
	}
	return nil
}

// removeLeft removes the stored files that are not final files, except those a removed blocking
// directory took along, and marks the directories on their way outside keep as maybe empty.
func (c *clearing) removeLeft(it *item) error {
	for rel := range it.stored {
		if _, ok := it.final[rel]; ok || hasParentIn(rel, c.blocking) {
			continue
		}
		if err := c.root.Remove(storeDir + "/" + rel); err != nil {
			return err
		}
		c.synced[path.Dir(rel)] = true
		for d := path.Dir(rel); !c.keep[d] && d != "."; d = path.Dir(d) {
			c.empty[d] = true
			c.synced[path.Dir(d)] = true
		}
	}
	return nil
}

// removeEmpty removes the maybe empty directories deepest first; a directory that still holds
// something is kept.
func (c *clearing) removeEmpty() error {
	for _, d := range slices.Backward(slices.Sorted(maps.Keys(c.empty))) {
		name := storeDir + "/" + d
		if entries, err := fs.ReadDir(c.root.FS(), name); err != nil || len(entries) > 0 {
			continue
		}
		if err := c.root.Remove(name); err != nil {
			return err
		}
		c.removed[d] = true
	}
	return nil
}

// sync makes the removals durable with an fsync of every changed directory that still exists.
func (c *clearing) sync() error {
	for _, d := range slices.Sorted(maps.Keys(c.synced)) {
		if c.removed[d] {
			continue // gone itself; its parent is in the set
		}
		if err := syncDir(c.root, storeDir+"/"+d); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// ensureDir creates a directory with mode and owner, or repairs both when they drifted.
func ensureDir(root *os.Root, name string, mode fs.FileMode, o Owner) error {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		if err := root.Mkdir(name, mode); err != nil {
			return err
		}
		if info, err = root.Lstat(name); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", display(name))
	}
	return setModeOwner(root, name, info, mode, o)
}

// syncService makes the directory of one service hold exactly want, with the given modes and
// owner (brief D4 and D7). It writes only what differs and touches nothing when nothing differs;
// a read-only directory is opened (0700) only while it changes.
func syncService(root *os.Root, dir string, want map[string][]byte, dirMode, fileMode fs.FileMode, o Owner) error {
	if err := prepareServiceDir(root, dir); err != nil {
		return err
	}
	existing, err := scan(root, dir)
	if err != nil {
		return err
	}
	s := &serviceSync{
		root: root, dir: dir, want: want, dirMode: dirMode, fileMode: fileMode, owner: o,
		existing: existing, needDirs: map[string]bool{},
	}
	for rel := range want {
		addParents(s.needDirs, rel)
	}
	if err := s.plan(); err != nil {
		return err
	}
	if err := s.openDirty(); err != nil {
		return err
	}
	if err := s.apply(); err != nil {
		return err
	}
	if err := s.syncDirty(); err != nil {
		return err
	}
	return s.setDirModes()
}

// prepareServiceDir makes sure the service directory is a directory: a missing one is created
// open (0700), and a link or file in its place is replaced, never followed.
func prepareServiceDir(root *os.Root, dir string) error {
	info, err := root.Lstat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return root.Mkdir(dir, openDirMode)
	case err != nil:
		return err
	case !info.IsDir():
		if err := root.Remove(dir); err != nil {
			return err
		}
		return root.Mkdir(dir, openDirMode)
	}
	return nil
}

// serviceSync is the state of syncService for one service directory; paths in the maps and lists
// are relative to dir.
type serviceSync struct {
	root              *os.Root
	dir               string
	want              map[string][]byte
	dirMode, fileMode fs.FileMode
	owner             Owner
	existing          map[string]fs.FileInfo // scan of dir before any change
	needDirs          map[string]bool        // directories on the way to a wanted file
	remove            []string               // topmost entries to remove
	mkdirs            []string               // needed directories to create
	writes            []string               // wanted files to write
	dirty             map[string]bool        // directories that get a new, renamed or removed child
}

// plan lists the changes, sorted, before the first one is made: entries that are neither a wanted
// file nor a needed directory, needed directories that are missing, and wanted files that are
// missing or differ in content, mode or owner.
func (s *serviceSync) plan() error {
	for _, rel := range slices.Sorted(maps.Keys(s.existing)) {
		if !s.keeps(rel) && !hasParentIn(rel, s.remove) {
			s.remove = append(s.remove, rel)
		}
	}
	for _, d := range slices.Sorted(maps.Keys(s.needDirs)) {
		if info, ok := s.existing[d]; !ok || !info.IsDir() {
			s.mkdirs = append(s.mkdirs, d)
		}
	}
	for _, rel := range slices.Sorted(maps.Keys(s.want)) {
		same, err := s.holds(rel)
		if err != nil {
			return err
		}
		if !same {
			s.writes = append(s.writes, rel)
		}
	}
	return nil
}

// keeps reports whether an existing entry stays: a wanted file that is no directory, or a needed
// directory that is one.
func (s *serviceSync) keeps(rel string) bool {
	info := s.existing[rel]
	_, wanted := s.want[rel]
	return (wanted && !info.IsDir()) || (s.needDirs[rel] && info.IsDir())
}

// holds reports whether the directory already has the wanted file rel: a regular file with the
// wanted mode, owner and content. The content is read only when mode and owner match.
func (s *serviceSync) holds(rel string) (bool, error) {
	info, ok := s.existing[rel]
	if !ok || !sameFileModeOwner(info, s.fileMode, s.owner) {
		return false, nil
	}
	current, err := readRegular(s.root, s.dir+"/"+rel, info)
	if err != nil {
		return false, err
	}
	return bytes.Equal(current, s.want[rel]), nil
}

// openDirty opens (0700) every existing directory that gets a new, renamed or removed child, so
// the running user can change it; a directory created below gets that mode anyway.
func (s *serviceSync) openDirty() error {
	s.dirty = map[string]bool{}
	for _, list := range [][]string{s.remove, s.mkdirs, s.writes} {
		for _, rel := range list {
			s.dirty[path.Dir(rel)] = true
		}
	}
	for _, d := range slices.Sorted(maps.Keys(s.dirty)) {
		if err := openExistingDir(s.root, path.Join(s.dir, d)); err != nil {
			return err
		}
	}
	return nil
}

// openExistingDir gives an existing directory the mode openDirMode when it has another one; a
// missing name is skipped, because it is created below with openDirMode.
func openExistingDir(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() == openDirMode {
		return nil
	}
	return root.Chmod(name, openDirMode)
}

// apply removes, then creates directories, then writes files, so a file can take the place of a
// removed directory and a directory the place of a removed file.
func (s *serviceSync) apply() error {
	for _, rel := range s.remove {
		if err := removeTree(s.root, s.dir+"/"+rel); err != nil {
			return err
		}
	}
	for _, d := range s.mkdirs {
		if err := s.root.Mkdir(s.dir+"/"+d, openDirMode); err != nil {
			return err
		}
	}
	for _, rel := range s.writes {
		name := s.dir + "/" + rel
		if err := writeAtomic(s.root, path.Dir(name), path.Base(name), s.want[rel], s.fileMode, s.owner); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// syncDirty makes the removals durable: after at least one removal it syncs every dirty
// directory. writeAtomic syncs the directory of each write itself.
func (s *serviceSync) syncDirty() error {
	if len(s.remove) == 0 {
		return nil
	}
	for _, d := range slices.Sorted(maps.Keys(s.dirty)) {
		if err := syncDir(s.root, path.Join(s.dir, d)); err != nil {
			return err
		}
	}
	return nil
}

// setDirModes gives every needed directory and the service directory its final mode and owner,
// deepest first and the service directory last, so an opened parent is closed after its children.
func (s *serviceSync) setDirModes() error {
	finalDirs := append(slices.Sorted(maps.Keys(s.needDirs)), ".")
	slices.SortStableFunc(finalDirs, func(a, b string) int { return depth(b) - depth(a) })
	for _, d := range finalDirs {
		name := path.Join(s.dir, d)
		info, err := s.root.Lstat(name)
		if err != nil {
			return err
		}
		if err := setModeOwner(s.root, name, info, s.dirMode, s.owner); err != nil {
			return err
		}
	}
	return nil
}

// depth orders directories for setDirModes: the number of slashes of a relative path, and -1 for
// the service directory itself.
func depth(d string) int {
	if d == "." {
		return -1
	}
	return strings.Count(d, "/")
}

// hasParentIn reports whether a directory in list contains rel; its removal takes rel along.
func hasParentIn(rel string, list []string) bool {
	for _, p := range list {
		if strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

// removeTree removes an entry and, for a directory, everything below it; read-only directories
// are opened first, because a service directory tree has mode 0500.
func removeTree(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if info.IsDir() {
		err := fs.WalkDir(root.FS(), name, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return root.Chmod(p, openDirMode)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return root.RemoveAll(name)
}

// report renders the items as lines with fingerprints of the final _store content.
func report(items []*item) Report {
	var r Report
	for _, it := range items {
		l := Line{Status: it.status, Name: it.name}
		if it.final != nil {
			l.Fingerprint = fingerprint(it.entry, it.final)
		}
		r.Lines = append(r.Lines, l)
	}
	return r
}

// fingerprint is "sha256:" plus 12 hex characters of the SHA-256 of the content; for a directory
// entry of the lines "<path inside the entry> <hex SHA-256 of the file>\n" sorted by path, so a
// change of any one file changes it.
func fingerprint(e Entry, files map[string][]byte) string {
	var data []byte
	if e.isDir() {
		var b strings.Builder
		for _, rel := range slices.Sorted(maps.Keys(files)) {
			sum := sha256.Sum256(files[rel])
			b.WriteString(strings.TrimPrefix(rel, e.Name) + " " + hex.EncodeToString(sum[:]) + "\n")
		}
		data = []byte(b.String())
	} else {
		for _, c := range files {
			data = c
		}
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])[:12]
}
