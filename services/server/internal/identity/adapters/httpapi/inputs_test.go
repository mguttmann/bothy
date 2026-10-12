// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// TestListUsers_RefusesBadQuery (red team M1-31, Major 2): NUL, invalid UTF-8 and control
// characters in q are 400, never 500; a forged cursor with such a handle is cursor_invalid.
func TestListUsers_RefusesBadQuery(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("anna", "active", false)
	for _, q := range []string{"%00", "a%00b", "%FF", "%C3", "%E2%80%AE"} {
		w.want(w.do("anna", http.MethodGet, usersPath+"?q="+q, "", ""), http.StatusBadRequest, "validation_failed")
	}
	anna := w.actors.Get("anna")
	binding := []byte(anna.ID.String() + "\x00q=\x00status=\x00sort=handle_asc")
	forged, err := httpserver.EncodeCursor("users", binding, map[string]string{"h": "a\x00b", "id": anna.ID.String()})
	if err != nil {
		t.Fatal(err)
	}
	w.want(w.do("anna", http.MethodGet, usersPath+"?cursor="+url.QueryEscape(forged), "", ""), http.StatusBadRequest, "cursor_invalid")
}

// TestProfile_BoundsAndStrictZones (red team M1-31, Majors 3 and 4, Minor 5): a locale longer
// than a BCP 47 tag needs is 400 too_long; time zones must be canonical IANA names; an empty
// locale or time zone on createUser is 400 like on updateUser.
func TestProfile_BoundsAndStrictZones(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	long := "de-x-" + strings.Repeat("abcdefgh-", 20) + "a"
	p := w.want(w.do("owner", http.MethodPatch, "/api/v1/me/profile", `{"locale":"`+long+`"}`, ""), http.StatusBadRequest, "validation_failed")
	if !strings.Contains(asString(p["errors"]), "too_long") {
		t.Errorf("errors %v, want too_long", p["errors"])
	}
	for _, tz := range []string{"Europe//Berlin", "./UTC", "posixrules", "right/Europe/Berlin", "posix/UTC", "europe/berlin", "utc", "Europe/Berlin/", "/UTC"} {
		w.want(w.do("owner", http.MethodPatch, "/api/v1/me/profile", `{"time_zone":"`+tz+`"}`, ""), http.StatusBadRequest, "validation_failed")
	}
	for _, tz := range []string{"Etc/GMT+1", "America/Argentina/Buenos_Aires", "UTC"} {
		w.want(w.do("owner", http.MethodPatch, "/api/v1/me/profile", `{"time_zone":"`+tz+`"}`, ""), http.StatusOK, "")
	}
	for _, body := range []string{`{"handle":"kim","display_name":"Kim","locale":""}`, `{"handle":"kim","display_name":"Kim","time_zone":""}`} {
		w.want(w.do("owner", http.MethodPost, usersPath, body, ""), http.StatusBadRequest, "validation_failed")
	}
}

func asString(v any) string { return fmt.Sprint(v) }

// TestListUsers_PagesPastLongHandles (red team M1-31 round 2, Major 1): a handle longer than the
// current principal.handle_max_chars (created under a larger limit) does not break the next page.
func TestListUsers_PagesPastLongHandles(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("anna", "active", false)
	w.person("bbbbbbbbbbbbbbb", "active", false) // 15 characters, the world allows 12
	w.person("carl", "active", false)
	m := w.want(w.do("anna", http.MethodGet, usersPath, "", ""), http.StatusOK, "")
	next, _ := m["next_cursor"].(string)
	if next == "" {
		t.Fatal("no next page")
	}
	p := w.want(w.do("anna", http.MethodGet, usersPath+"?cursor="+url.QueryEscape(next), "", ""), http.StatusOK, "")
	if items := p["items"].([]any); len(items) != 1 || items[0].(map[string]any)["handle"] != "carl" {
		t.Fatalf("second page %v", items)
	}
}
