// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// consoleIndex is the index.html of the fixture build, with the nonce placeholder in the
// bootstrap script tag as in apps/console/web/index.html.
const consoleIndex = "<!DOCTYPE html><html><head></head><body><script src=\"/console/flutter_bootstrap.js\" async nonce=\"" +
	httpserver.NoncePlaceholder + "\"></script></body></html>"

// isIndex reports whether body is consoleIndex with some nonce in place of the placeholder.
func isIndex(body string) bool {
	m := bodyNonce.FindStringSubmatch(body)
	return m != nil && m[1] != "" && body == strings.Replace(consoleIndex, httpserver.NoncePlaceholder, m[1], 1)
}

// writeIndex writes index.html with body into dir.
func writeIndex(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// consoleBuild writes a small console build into a temporary directory: index.html, the files a
// Flutter web build has (main.dart.js, CanvasKit, a font, version.json), a hashed asset and a
// file outside the build that a traversal would reach.
func consoleBuild(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	dir := filepath.Join(parent, "web")
	files := map[string]string{
		"index.html":                       consoleIndex,
		"main.dart.js":                     "console.log('main');",
		"flutter_bootstrap.js":             "_flutter.loader.load({});",
		"canvaskit/canvaskit.wasm":         "\x00asm",
		"assets/fonts/MaterialIcons.otf":   "otf",
		"assets/packages/x/font.ttf":       "ttf",
		"version.json":                     `{"spec_version":"0.2.0"}`,
		"main.3f9a1c2b7d.dart.js":          "hashed",
		"assets/AssetManifest.bin":         "bin",
		"assets/shaders/ink_sparkle.frag":  "frag",
		"icons/Icon-192.png":               "png",
		"favicon.svg":                      "<svg/>",
		"styles/console.css":               "body{}",
		"assets/fonts/AtkinsonNext.woff2":  "woff2",
		"canvaskit/canvaskit.js.symbols":   "symbols",
		"canvaskit/chromium/canvaskit.mjs": "export {}",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(parent, "secret.txt"), filepath.Join(dir, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// consoleServer serves the console of dir through the public chain, the same way the api process
// does: the console is mounted on the mux beside the REST routes.
func consoleServer(t *testing.T, dir string) http.Handler {
	t.Helper()
	v, err := httpserver.NewValidator(httpservertest.Contract, nil)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := httpserver.NewChain(httpserver.DefaultChainConfig(httpservertest.PublicURL),
		httpserver.Deps{Authenticator: actor.AnonymousOnly{}, Validator: v})
	if err != nil {
		t.Fatal(err)
	}
	assets, err := httpserver.OpenConsoleAssets(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = assets.Close() })
	mux := http.NewServeMux()
	assets.Mount(mux)
	return chain.Public(mux)
}

// answer is one recorded response: status, header and body.
type answer struct {
	StatusCode int
	Header     http.Header
}

// doWith sends a GET of target with the extra headers to h.
func doWith(t *testing.T, h http.Handler, target string, headers map[string]string) (answer, string) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return answer{StatusCode: rec.Code, Header: rec.Header()}, rec.Body.String()
}

// do sends one request to h and returns the recorded status and header with the body.
func do(t *testing.T, h http.Handler, method, target string) (answer, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return answer{StatusCode: rec.Code, Header: rec.Header()}, rec.Body.String()
}

// TestConsoleServing holds the serving rules of the console (brief M6-02 Design note 4): exact
// files with their content type and cache rule, the SPA fallback to index.html only below
// /console/, /api/ untouched, traversal refused before the file system, the headers of SEC-HDR-1
// and of SEC-HDR-3 (the CSP itself in csp_test.go), and 404 when console.assets_dir is empty.
func TestConsoleServing(t *testing.T) {
	t.Parallel()
	h := consoleServer(t, consoleBuild(t))

	t.Run("files with type and cache rule", func(t *testing.T) {
		t.Parallel()
		cases := []struct{ path, body, typ, cache string }{
			{"/console/", consoleIndex, "text/html; charset=utf-8", "no-store"},
			{"/console/index.html", consoleIndex, "text/html; charset=utf-8", "no-store"},
			{"/console/main.dart.js", "console.log('main');", "text/javascript; charset=utf-8", "no-cache"},
			{"/console/flutter_bootstrap.js", "_flutter.loader.load({});", "text/javascript; charset=utf-8", "no-cache"},
			{"/console/canvaskit/canvaskit.wasm", "\x00asm", "application/wasm", "no-cache"},
			{"/console/canvaskit/chromium/canvaskit.mjs", "export {}", "text/javascript; charset=utf-8", "no-cache"},
			{"/console/assets/fonts/MaterialIcons.otf", "otf", "font/otf", "no-cache"},
			{"/console/assets/packages/x/font.ttf", "ttf", "font/ttf", "no-cache"},
			{"/console/assets/fonts/AtkinsonNext.woff2", "woff2", "font/woff2", "no-cache"},
			{"/console/version.json", `{"spec_version":"0.2.0"}`, "application/json", "no-cache"},
			{"/console/main.3f9a1c2b7d.dart.js", "hashed", "text/javascript; charset=utf-8", "public, max-age=31536000, immutable"},
			{"/console/assets/AssetManifest.bin", "bin", "application/octet-stream", "no-cache"},
			{"/console/assets/shaders/ink_sparkle.frag", "frag", "application/octet-stream", "no-cache"},
			{"/console/canvaskit/canvaskit.js.symbols", "symbols", "application/octet-stream", "no-cache"},
			{"/console/icons/Icon-192.png", "png", "image/png", "no-cache"},
			{"/console/favicon.svg", "<svg/>", "image/svg+xml", "no-cache"},
			{"/console/styles/console.css", "body{}", "text/css; charset=utf-8", "no-cache"},
		}
		for _, c := range cases {
			res, body := do(t, h, http.MethodGet, c.path)
			if res.StatusCode != http.StatusOK || (body != c.body && (c.body != consoleIndex || !isIndex(body))) {
				t.Errorf("GET %s = %d %q, want 200 %q", c.path, res.StatusCode, body, c.body)
			}
			if got := res.Header.Get("Content-Type"); got != c.typ {
				t.Errorf("GET %s Content-Type = %q, want %q", c.path, got, c.typ)
			}
			if got := res.Header.Get("Cache-Control"); got != c.cache {
				t.Errorf("GET %s Cache-Control = %q, want %q", c.path, got, c.cache)
			}
		}
	})

	t.Run("SPA fallback only below /console/", func(t *testing.T) {
		t.Parallel()
		for _, p := range []string{
			"/console/login", "/console/login/callback?login_code=lc_1&login_attempt_id=a",
			"/console/step-up/callback?challenge_id=c", "/console/stats", "/console/people/x/y",
			"/console/missing.js", "/console/assets/", "/console/canvaskit",
		} {
			res, body := do(t, h, http.MethodGet, p)
			if res.StatusCode != http.StatusOK || !isIndex(body) {
				t.Errorf("GET %s = %d %q, want index.html", p, res.StatusCode, body)
			}
			if got := res.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("GET %s Cache-Control = %q, want no-store", p, got)
			}
		}
		for _, p := range []string{"/api/v1/console/session", "/api/v1/unknown", "/", "/consolex", "/index.html", "/main.dart.js"} {
			res, body := do(t, h, http.MethodGet, p)
			if res.StatusCode != http.StatusNotFound || !strings.Contains(body, `"code":"not_found"`) {
				t.Errorf("GET %s = %d %q, want a 404 problem", p, res.StatusCode, body)
			}
		}
	})

	t.Run("redirect without slash", func(t *testing.T) {
		t.Parallel()
		res, _ := do(t, h, http.MethodGet, "/console")
		if res.StatusCode != http.StatusMovedPermanently || res.Header.Get("Location") != "/console/" {
			t.Errorf("GET /console = %d Location %q, want 301 /console/", res.StatusCode, res.Header.Get("Location"))
		}
	})

	t.Run("HEAD and other methods", func(t *testing.T) {
		t.Parallel()
		res, body := do(t, h, http.MethodHead, "/console/")
		if res.StatusCode != http.StatusOK || body != "" || res.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("HEAD /console/ = %d %q %q", res.StatusCode, body, res.Header.Get("Cache-Control"))
		}
		res, body = do(t, h, http.MethodPost, "/console/login")
		if res.StatusCode != http.StatusMethodNotAllowed || !strings.Contains(body, `"code":"method_not_allowed"`) {
			t.Errorf("POST /console/login = %d %q, want a 405 problem", res.StatusCode, body)
		}
	})

	t.Run("traversal refused before the file system", func(t *testing.T) {
		t.Parallel()
		for _, p := range []string{
			"/console/../secret.txt", "/console/%2e%2e/secret.txt", "/console/assets/..%2f..%2fsecret.txt",
			"/console/assets%2fAssetManifest.bin", "/console/assets%5cAssetManifest.bin",
			"/console/assets\\AssetManifest.bin", "/console/escape.txt", "/console/x%00.js",
		} {
			res, body := do(t, h, http.MethodGet, p)
			if res.StatusCode != http.StatusNotFound || strings.Contains(body, "secret") || isIndex(body) {
				t.Errorf("GET %s = %d %q, want 404 without content", p, res.StatusCode, body)
			}
		}
	})

	t.Run("security headers", func(t *testing.T) {
		t.Parallel()
		for _, p := range []string{"/console/", "/console/main.dart.js", "/console/stats"} {
			res, _ := do(t, h, http.MethodGet, p)
			want := map[string]string{
				"X-Content-Type-Options":     "nosniff",
				"Referrer-Policy":            "no-referrer",
				"Strict-Transport-Security":  "max-age=31536000",
				"Cross-Origin-Opener-Policy": "same-origin",
				"X-Frame-Options":            "DENY",
				"Permissions-Policy":         "camera=(), microphone=(), geolocation=(), payment=(), usb=()",
			}
			for k, v := range want {
				if got := res.Header.Get(k); got != v {
					t.Errorf("GET %s %s = %q, want %q", p, k, got, v)
				}
			}
			// The CSP and COEP of M6-03 are held by TestCSPHeader and TestConsoleIsolationHeaders.
			for _, k := range []string{"Server"} {
				if got := res.Header.Get(k); got != "" {
					t.Errorf("GET %s %s = %q, want none", p, k, got)
				}
			}
		}
	})

	t.Run("revalidation by content, never by modification time", func(t *testing.T) {
		t.Parallel()
		// A rollback to an image built earlier serves older files: a browser that sends the
		// later If-Modified-Since must still get the new content (review finding M6-02).
		future := time.Now().Add(24 * time.Hour).UTC().Format(http.TimeFormat)
		for _, p := range []string{"/console/main.dart.js", "/console/version.json", "/console/"} {
			res, body := doWith(t, h, p, map[string]string{"If-Modified-Since": future})
			if res.StatusCode != http.StatusOK || body == "" {
				t.Errorf("GET %s with a later If-Modified-Since = %d, want 200 with the content", p, res.StatusCode)
			}
			if res.Header.Get("Last-Modified") != "" {
				t.Errorf("GET %s sends Last-Modified %q", p, res.Header.Get("Last-Modified"))
			}
		}
		res, _ := do(t, h, http.MethodGet, "/console/main.dart.js")
		etag := res.Header.Get("ETag")
		if !strings.HasPrefix(etag, `"`) || len(etag) < 10 {
			t.Fatalf("ETag = %q, want a strong content tag", etag)
		}
		res, body := doWith(t, h, "/console/main.dart.js", map[string]string{"If-None-Match": etag})
		if res.StatusCode != http.StatusNotModified || body != "" {
			t.Errorf("GET with the current ETag = %d %q, want 304", res.StatusCode, body)
		}
		other, _ := do(t, h, http.MethodGet, "/console/flutter_bootstrap.js")
		if other.Header.Get("ETag") == etag {
			t.Error("two files with different content share an ETag")
		}
	})

	t.Run("empty assets dir disables the console", func(t *testing.T) {
		t.Parallel()
		off := consoleServer(t, "")
		for _, p := range []string{"/console/", "/console", "/console/login"} {
			res, body := do(t, off, http.MethodGet, p)
			if res.StatusCode != http.StatusNotFound || !strings.Contains(body, `"code":"not_found"`) {
				t.Errorf("GET %s without console = %d %q, want a 404 problem", p, res.StatusCode, body)
			}
		}
	})
}

// TestOpenConsoleAssetsRefusesABuildWithoutIndex: a configured directory that is missing or holds
// no index.html ends the start instead of answering every console path with 404.
func TestOpenConsoleAssetsRefusesABuildWithoutIndex(t *testing.T) {
	t.Parallel()
	if _, err := httpserver.OpenConsoleAssets(filepath.Join(t.TempDir(), "missing"), ""); err == nil {
		t.Error("a missing directory was accepted")
	}
	if _, err := httpserver.OpenConsoleAssets(t.TempDir(), ""); err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Errorf("a directory without index.html: err = %v, want one naming index.html", err)
	}
}
