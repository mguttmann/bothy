// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// ConsolePrefix is the path below which the api process serves the web console (brief M6-02,
// docs/spec/15-client-architektur.md 11): the same origin as /api/, so the console cookie and
// the CSRF header of M6-01 work without CORS.
const ConsolePrefix = "/console/"

// consoleIndex is the entry document of the Flutter build; every path below ConsolePrefix that
// is not a file of the build gets it (SPA fallback), so a reload of /console/stats starts the app.
const consoleIndex = "index.html"

// Cache rules of the console (docs/DECISIONS.md D-M6-02-4): index.html is never stored, because
// it names the loader and a stale copy would pair an old app with a new API; a file whose name
// carries a content hash never changes and may be kept; every other file is revalidated by its
// content tag (ETag), never by its modification time.
const (
	cacheConsoleIndex  = "no-store"
	cacheConsoleHashed = "public, max-age=31536000, immutable"
	cacheConsoleOther  = "no-cache"
)

// The headers of SEC-HDR-3 that M6-02 sets (docs/spec/19-sicherheit.md 3). The CSP and
// Cross-Origin-Embedder-Policy come with M6-03; until then SEC-HDR-3 is incomplete
// (docs/DECISIONS.md D-M6-02-5).
const (
	headerCOOP              = "Cross-Origin-Opener-Policy"
	headerFrameOptions      = "X-Frame-Options"
	headerPermissionsPolicy = "Permissions-Policy"
	valueFrameDeny          = "DENY"
	valuePermissionsPolicy  = "camera=(), microphone=(), geolocation=(), payment=(), usb=()"
)

// hashedName matches a file name with a content hash of at least 8 hex digits between dots, for
// example main.3f9a1c2b.dart.js. Flutter itself names no file this way today; the rule keeps a
// fingerprinted asset cacheable as soon as the build produces one.
var hashedName = regexp.MustCompile(`\.[0-9a-f]{8,}\.`)

// consoleTypes are the content types of the files a Flutter web build writes. The table is
// fixed instead of the system's MIME database, because the distroless image has none and the
// answer must not depend on the host; X-Content-Type-Options: nosniff (SEC-HDR-1) makes the
// browser trust it.
var consoleTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".json":  "application/json",
	".wasm":  "application/wasm",
	".png":   "image/png",
	".svg":   "image/svg+xml",
	".ico":   "image/x-icon",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".woff2": "font/woff2",
}

// ConsoleAssets serves the built web console (apps/console, `flutter build web`) from the
// directory of console.assets_dir under ConsolePrefix. A nil *ConsoleAssets is a disabled console:
// Mount adds no route, so every path below /console/ is the 404 of the chain. Open it once with
// OpenConsoleAssets and Close it on shutdown; it is safe for concurrent use.
//
// Example:
//
//	assets, err := httpserver.OpenConsoleAssets(cfg.AssetsDir) // "" disables the console
//	if err != nil {
//		return err
//	}
//	assets.Mount(publicMux)
type ConsoleAssets struct {
	root *os.Root
	// etags caches the content tag per file version (etagKey); the build is immutable in the
	// image, so each file is hashed once.
	etags sync.Map
}

// etagKey names one version of a file of the build.
type etagKey struct {
	rel  string
	size int64
	mod  time.Time
}

// OpenConsoleAssets opens dir as the root of the console build. An empty dir returns a nil
// *ConsoleAssets and no error (console disabled). The error names the directory when it cannot
// be opened or holds no regular index.html, so a broken image ends the start instead of serving
// 404 for every console path.
func OpenConsoleAssets(dir string) (*ConsoleAssets, error) {
	if dir == "" {
		return nil, nil
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("httpserver: console.assets_dir %s: %w", dir, err)
	}
	info, err := root.Stat(consoleIndex)
	if err != nil || !info.Mode().IsRegular() {
		_ = root.Close()
		return nil, fmt.Errorf("httpserver: console.assets_dir %s holds no regular %s", dir, consoleIndex)
	}
	return &ConsoleAssets{root: root}, nil
}

// Close releases the directory; a nil *ConsoleAssets has nothing to close.
func (a *ConsoleAssets) Close() error {
	if a == nil {
		return nil
	}
	return a.root.Close()
}

// Mount registers GET (and with it HEAD) of ConsolePrefix and of /console on mux; /console
// redirects to /console/. It is a no-op for a nil *ConsoleAssets. The routes have no operation of
// the contract, so the chain runs steps 1 to 4 for them (SEC-HDR-1, trace, client IP, setup gate,
// 404 and 405 problems) and no per-route stage (docs/DECISIONS.md D-M6-02-4).
func (a *ConsoleAssets) Mount(mux interface {
	Handle(pattern string, handler http.Handler)
},
) {
	if a == nil {
		return
	}
	mux.Handle(http.MethodGet+" "+strings.TrimSuffix(ConsolePrefix, "/"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		consoleHeaders(w.Header())
		http.Redirect(w, r, ConsolePrefix, http.StatusMovedPermanently)
	}))
	mux.Handle(http.MethodGet+" "+ConsolePrefix, http.HandlerFunc(a.serve))
}

// serve answers one request below ConsolePrefix: the exact file of the build, index.html for a
// path the build does not hold, and 404 for a path that tries to leave the build directory.
func (a *ConsoleAssets) serve(w http.ResponseWriter, r *http.Request) {
	consoleHeaders(w.Header())
	rel, ok := consoleRelPath(r)
	if !ok {
		writeProblem(w, r, errs.New(errs.CodeNotFound, ""))
		return
	}
	if rel != "" && a.serveFile(w, r, rel) {
		return
	}
	if !a.serveFile(w, r, consoleIndex) {
		writeProblem(w, r, errs.New(errs.CodeNotFound, ""))
	}
}

// consoleRelPath returns the path of r relative to the build directory, "" for ConsolePrefix
// itself. It refuses (ok false) every path that is not plain before any file system call: an
// encoded slash or backslash, a backslash, a NUL byte or a "." or ".." segment. The chain already
// answers paths that are not in clean form with 404 (dispatchMux); this check does not rely on it.
func consoleRelPath(r *http.Request) (string, bool) {
	raw := strings.ToLower(r.URL.EscapedPath())
	if strings.Contains(raw, "%2f") || strings.Contains(raw, "%5c") {
		return "", false
	}
	p := r.URL.Path
	if !strings.HasPrefix(p, ConsolePrefix) || strings.ContainsAny(p, "\\\x00") {
		return "", false
	}
	rel := strings.TrimPrefix(p, ConsolePrefix)
	for seg := range strings.SplitSeq(rel, "/") {
		if seg == "." || seg == ".." {
			return "", false
		}
	}
	return strings.TrimSuffix(rel, "/"), true
}

// serveFile writes the regular file rel of the build with its content type and cache rule and
// reports true; it reports false without writing when rel is no regular file of the build. The
// root refuses names that leave the directory, also through a symbolic link; such a name is
// answered with 404 here, never with index.html.
func (a *ConsoleAssets) serveFile(w http.ResponseWriter, r *http.Request, rel string) bool {
	f, err := a.root.Open(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false
		}
		writeProblem(w, r, errs.New(errs.CodeNotFound, ""))
		return true
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	name := path.Base(rel)
	h := w.Header()
	h.Set(headerContentType, consoleType(name))
	switch {
	case rel == consoleIndex:
		h.Set(headerCacheControl, cacheConsoleIndex)
	case hashedName.MatchString(name):
		h.Set(headerCacheControl, cacheConsoleHashed)
	default:
		h.Set(headerCacheControl, cacheConsoleOther)
	}
	etag, err := a.etag(f, etagKey{rel: rel, size: info.Size(), mod: info.ModTime()})
	if err != nil {
		writeProblem(w, r, errs.Wrap(errs.CodeInternalError, err, ""))
		return true
	}
	h.Set(headerETag, etag)
	// The zero time: no Last-Modified and no 304 by If-Modified-Since. A rollback serves files
	// older than the ones a browser holds, so only the content tag may decide (D-M6-02-4).
	http.ServeContent(w, r, name, time.Time{}, f)
	return true
}

// etag returns the strong content tag of f (the first 16 bytes of its SHA-256 in hex, quoted)
// and leaves f at its start.
func (a *ConsoleAssets) etag(f *os.File, key etagKey) (string, error) {
	if v, ok := a.etags.Load(key); ok {
		return v.(string), nil
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	tag := `"` + hex.EncodeToString(sum.Sum(nil)[:16]) + `"`
	a.etags.Store(key, tag)
	return tag, nil
}

// consoleType returns the content type of a file name of the build, application/octet-stream
// for an extension the table does not know (AssetManifest.bin, shaders, symbol files).
func consoleType(name string) string {
	if t, ok := consoleTypes[strings.ToLower(path.Ext(name))]; ok {
		return t
	}
	return "application/octet-stream"
}

// consoleHeaders sets the headers of SEC-HDR-3 that this ticket owns on every console response.
// SEC-HDR-1 comes from the recover step of the chain.
func consoleHeaders(h http.Header) {
	h.Set(headerCOOP, valueSameOrigin)
	h.Set(headerFrameOptions, valueFrameDeny)
	h.Set(headerPermissionsPolicy, valuePermissionsPolicy)
}
