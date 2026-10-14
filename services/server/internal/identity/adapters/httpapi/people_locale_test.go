// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestCreateUser_UsesLocaleDefault: a person created without locale gets locale.default of the
// instance settings (PLAN row M1-46), not the column default of 03; an explicit locale wins.
func TestCreateUser_UsesLocaleDefault(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.person("owner", "active", true)
	w.sql(`INSERT INTO instance_settings (key, value) VALUES ('locale.default', '"en"')`)
	w.want(w.do("owner", http.MethodPost, usersPath, `{"handle":"eve","display_name":"Eve"}`, ""), http.StatusCreated, "")
	if n := w.count(`SELECT count(*) FROM users u JOIN principals p ON p.id = u.principal_id WHERE p.handle = 'eve' AND u.locale = 'en'`); n != 1 {
		t.Errorf("stored locale is not en (%d rows)", n)
	}
	w.want(w.do("owner", http.MethodPost, usersPath, `{"handle":"sam","display_name":"Sam","locale":"de-AT"}`, ""), http.StatusCreated, "")
	if n := w.count(`SELECT count(*) FROM users u JOIN principals p ON p.id = u.principal_id WHERE p.handle = 'sam' AND u.locale = 'de-AT'`); n != 1 {
		t.Errorf("explicit locale not stored (%d rows)", n)
	}
}

// TestGetInstance_AgentToken: getInstance serves every active principal, an agent token
// included (05 2.6: instance.read needs no scope); the body is the same as for a person.
func TestGetInstance_AgentToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.sql(`INSERT INTO instance (instance_id, schema_version, sync_epoch, setup_completed_at) VALUES ($1, 14, $2, now())`,
		ids.New().String(), ids.New().String())
	w.agent("helper")
	w.person("anna", "active", false)
	r := w.do("helper", http.MethodGet, "/api/v1/instance", "", "")
	m := w.want(r, http.StatusOK, "")
	if !registeredFlagsOff(m["features"]) {
		t.Errorf("features %v, want the registered flags off (M1-47)", m["features"])
	}
	person := w.do("anna", http.MethodGet, "/api/v1/instance", "", "")
	w.want(person, http.StatusOK, "")
	if r.Header.Get("ETag") == "" || r.Header.Get("ETag") != person.Header.Get("ETag") {
		t.Errorf("ETag agent %q, person %q; the body is the same", r.Header.Get("ETag"), person.Header.Get("ETag"))
	}
}
