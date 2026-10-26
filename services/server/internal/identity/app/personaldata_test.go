// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
)

// releasingFiles is the avatar port of the module files for the account deletion: it only
// records what Release gets.
type releasingFiles struct {
	identity.AvatarFiles
	released []ids.ID
}

func (f *releasingFiles) Release(_ context.Context, fileID, uploadedBy ids.ID) error {
	if uploadedBy.IsZero() {
		return errs.New(errs.CodeInternalError, "release without uploader")
	}
	f.released = append(f.released, fileID)
	return nil
}

// personalWorld is the lifecycle world with the participant of identity and the rows of a
// person (alex or sam) that 20 section 2 lists for identity.
type personalWorld struct {
	*lifeWorld
	pd       *app.PersonalData
	files    *releasingFiles
	provider ids.ID
}

func newPersonalWorld(t *testing.T) *personalWorld {
	t.Helper()
	w := &personalWorld{lifeWorld: newLifeWorld(t), files: &releasingFiles{}, provider: ids.New()}
	w.sql(`INSERT INTO identity_providers (id, kind, name, config) VALUES ($1, 'local', 'Local accounts', '{}')`, w.provider.String())
	w.p.BindAvatarFiles(w.files)
	p, err := app.NewPersonalData(w.l, w.repo)
	if err != nil {
		t.Fatal(err)
	}
	w.pd = p
	return w
}

// personRows are the identity rows of one person that the tests insert.
type personRows struct {
	id                     ids.ID
	avatar                 ids.ID
	device, heldDevice     ids.ID
	session, heldSession   ids.ID
	identity, passkey      ids.ID
	token, code            ids.ID
	revokedOwner, ownerRow ids.ID
}

// rows inserts for the person id: an avatar, profile values, two owners rows (one revoked), a
// local identity with password and TOTP, a passkey, an active device with a key and a session
// with a share_ext child, a refresh family and token, a step-up challenge, a personal API token,
// an enrollment code with a passkey enrollment challenge, a login flow, a completed pairing and,
// when held, a second device and session that a closed terminal session of terminal holds.
func (w *personalWorld) rows(id ids.ID, handle string, held bool) personRows {
	w.t.Helper()
	r := personRows{id: id, avatar: ids.New(), identity: ids.New(), passkey: ids.New(), code: ids.New()}
	w.sql(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose)
		VALUES ($1, $3, '\x00', 1, 'image/png', 'avatar', $2, 'avatar')`, r.avatar.String(), id.String(), "avatars/"+r.avatar.String())
	w.sql(`UPDATE principals SET avatar_file_id = $2 WHERE id = $1`, id.String(), r.avatar.String())
	w.sql(`UPDATE users SET locale = 'en', time_zone = 'Europe/Berlin', last_active_at = $2 WHERE principal_id = $1`, id.String(), peopleStart)
	r.revokedOwner, r.ownerRow = ids.New(), ids.New()
	w.sql(`INSERT INTO owners (id, user_id, reason, granted_at, revoked_at) VALUES ($1, $2, 'interim', $3, $4)`,
		r.revokedOwner.String(), id.String(), peopleStart.Add(-48*time.Hour), peopleStart.Add(-24*time.Hour))
	w.sql(`INSERT INTO owners (id, user_id, reason, granted_at) VALUES ($1, $2, 'runs the instance', $3)`,
		r.ownerRow.String(), id.String(), peopleStart.Add(-time.Hour))
	w.sql(`INSERT INTO user_identities (id, user_id, provider_id, subject, email_at_link, linked_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		r.identity.String(), id.String(), w.provider.String(), handle, handle+"@example.com", peopleStart)
	w.sql(`INSERT INTO local_credentials (identity_id, password_hash, totp_secret_ciphertext, totp_key_ref, totp_enabled_at)
		VALUES ($1, 'argon2id-secret-hash', '\xdeadbeef', 'k1', $2)`, r.identity.String(), peopleStart)
	w.sql(`INSERT INTO passkey_credentials (id, user_id, credential_id, public_key_cose, sign_count, transports, name)
		VALUES ($1, $2, $3, '\xc0ffee', 7, '{internal}', 'Laptop')`, r.passkey.String(), id.String(), r.passkey[:])
	r.device = w.device(id, "active", peopleStart)
	r.session = w.session(id, r.device, ids.ID{})
	w.session(id, r.device, r.session)
	w.sql(`UPDATE auth_sessions SET last_ip = '192.0.2.10' WHERE id = $1`, r.session.String())
	w.sql(`INSERT INTO refresh_tokens (id, family_id, generation, token_hash, expires_at)
		SELECT $1, f.id, 1, $2, $3 FROM refresh_token_families f WHERE f.session_id = $4`,
		ids.New().String(), []byte(ids.New().String()), peopleStart.Add(time.Hour), r.session.String())
	w.sql(`INSERT INTO auth_challenges (id, purpose, session_id, challenge_hash, expires_at) VALUES ($1, 'step_up', $2, $3, $4)`,
		ids.New().String(), r.session.String(), []byte(ids.New().String()), peopleStart.Add(time.Hour))
	r.token = ids.New()
	w.sql(`INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, expires_at, created_by)
		VALUES ($1, $2, 'cli', 'bothy_pat_x', $3, $4, $2)`, r.token.String(), id.String(), r.token[:], peopleStart.Add(24*time.Hour))
	w.sql(`INSERT INTO enrollment_codes (id, user_id, purpose, code_hash, expires_at) VALUES ($1, $2, 'recovery', $3, $4)`,
		r.code.String(), id.String(), r.code[:], peopleStart.Add(time.Hour))
	w.sql(`INSERT INTO auth_challenges (id, purpose, enrollment_code_id, challenge_hash, expires_at) VALUES ($1, 'passkey_enroll', $2, $3, $4)`,
		ids.New().String(), r.code.String(), []byte(ids.New().String()), peopleStart.Add(time.Hour))
	w.sql(`INSERT INTO auth_login_flows (id, provider_id, mode, user_id, oidc_state_hash, oidc_nonce_hash, client_challenge, app_redirect_uri, expires_at)
		VALUES ($1, $2, 'link', $3, $4, '\x01', 'challenge', 'app://callback', $5)`,
		ids.New().String(), w.provider.String(), id.String(), []byte(ids.New().String()), peopleStart.Add(time.Hour))
	w.sql(`INSERT INTO device_pairings (id, pairing_code_hash, device_registration, status, user_id, device_id, expires_at, completed_at)
		VALUES ($1, $2, '{}', 'completed', $3, $4, $5, $5)`, ids.New().String(), []byte(ids.New().String()), id.String(), r.device.String(), peopleStart)
	if held {
		r.heldDevice = w.device(id, "active", peopleStart)
		w.sql(`UPDATE devices SET app_version = '1.2.3', name = $2 WHERE id = $1`, r.heldDevice.String(), "Phone of "+handle)
		r.heldSession = w.session(id, r.heldDevice, ids.ID{})
		w.sql(`UPDATE auth_sessions SET last_ip = '192.0.2.20' WHERE id = $1`, r.heldSession.String())
		host, hsu := ids.New(), ids.New()
		w.sql(`INSERT INTO hosts (id, name, state, pairing_code_hash, created_by) VALUES ($1, 'build host', 'pairing', $2, $3)`,
			host.String(), []byte(host.String()), id.String())
		w.sql(`INSERT INTO host_session_users (id, host_id, user_id, os_user, created_by) VALUES ($1, $2, $3, 'dev', $3)`,
			hsu.String(), host.String(), id.String())
		w.sql(`INSERT INTO terminal_sessions (id, host_id, user_id, host_session_user_id, opened_by_device_id, device_id, auth_session_id,
			profile, state, token_jti, key_generation, closed_at, close_reason)
			VALUES ($1, $2, $3, $4, $5, $5, $6, 'shell', 'closed', $7, 1, $8, 'logout')`,
			ids.New().String(), host.String(), id.String(), hsu.String(), r.heldDevice.String(), r.heldSession.String(),
			ids.New().String(), peopleStart)
	}
	return r
}

// exportSchema compiles docs/api/gdpr-export/v1/sections/<name>.schema.json.
func exportSchema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	name += ".schema.json"
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../docs/api/gdpr-export/v1/sections", name)) // #nosec G304 -- a schema file of the repository
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(name, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// exportSection validates the section key of f against its schema and returns it decoded.
func exportSection(t *testing.T, w *personaldata.MemoryWriter, f personaldata.File, key string) any {
	t.Helper()
	raw, ok := w.Sections[f][key]
	if !ok {
		t.Fatalf("section %s of %s is missing", key, f)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	stem := strings.TrimSuffix(string(f), ".json")
	if err := exportSchema(t, stem+"."+key).Validate(v); err != nil {
		t.Fatalf("section %s of %s does not match its schema: %v\n%s", key, f, err, raw)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// members returns the sorted member names of the JSON object v.
func members(t *testing.T, v any) []string {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is no object", v)
	}
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// list returns the objects of the JSON array v.
func list(t *testing.T, v any) []map[string]any {
	t.Helper()
	a, ok := v.([]any)
	if !ok {
		t.Fatalf("%v is no array", v)
	}
	out := make([]map[string]any, 0, len(a))
	for _, e := range a {
		out = append(out, e.(map[string]any))
	}
	return out
}

// TestPersonalDataExport (M7-06, 20 section 5.2): the export of identity holds exactly Alex's
// rows with the documented members, valid against the schemas of docs/api/gdpr-export/v1, no
// row of Sam and no hash, key, credential id, secret or sign count.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	w := newPersonalWorld(t)
	alex := w.person("alex", "active", false)
	sam := w.person("sam", "active", false)
	a := w.rows(alex.ID, "alex", true)
	s := w.rows(sam.ID, "sam", false)

	out := personaldata.NewMemoryWriter()
	if err := w.pd.Export(context.Background(), personaldata.Export{Subject: alex.ID}, out); err != nil {
		t.Fatal(err)
	}

	principal := exportSection(t, out, personaldata.FileProfile, "principal")
	if got := members(t, principal); !slices.Equal(got, []string{"avatar_file_id", "created_at", "display_name", "handle", "id", "kind", "status"}) {
		t.Errorf("principal members %v", got)
	}
	if p := principal.(map[string]any); p["id"] != alex.ID.String() || p["handle"] != "alex" || p["avatar_file_id"] != a.avatar.String() {
		t.Errorf("principal %v", p)
	}
	user := exportSection(t, out, personaldata.FileProfile, "user")
	if got := members(t, user); !slices.Equal(got, []string{"email", "last_active_at", "locale", "time_zone"}) {
		t.Errorf("user members %v", got)
	}
	if u := user.(map[string]any); u["email"] != "alex@example.com" || u["locale"] != "en" || u["time_zone"] != "Europe/Berlin" {
		t.Errorf("user %v", u)
	}

	want := map[string]struct {
		f       personaldata.File
		n       int
		members []string
		id      string
	}{
		"identities": {personaldata.FileIdentities, 1, []string{"email_at_link", "id", "linked_at", "provider_id", "subject"}, a.identity.String()},
		"passkeys":   {personaldata.FileIdentities, 1, []string{"created_at", "id", "name", "transports"}, a.passkey.String()},
		"local_credentials": {personaldata.FileIdentities, 1,
			[]string{"identity_id", "password_changed_at", "totp_enabled_at", "totp_enrolled"}, ""},
		"devices": {personaldata.FileDevices, 2, []string{"approved_at", "created_at", "id", "last_seen_at", "name", "platform", "state"}, a.device.String()},
		// The session with a last IP; the share_ext child and the held session follow.
		"sessions":   {personaldata.FileDevices, 3, []string{"auth_method", "client_component", "created_at", "device_id", "expires_at", "id", "last_ip"}, a.session.String()},
		"api_tokens": {personaldata.FileDevices, 1, []string{"created_at", "expires_at", "id", "name"}, a.token.String()},
		"owner":      {personaldata.FileAccess, 2, []string{"granted_at", "id", "reason", "revoked_at", "role"}, a.revokedOwner.String()},
	}
	for key, c := range want {
		rows := list(t, exportSection(t, out, c.f, key))
		if len(rows) != c.n {
			t.Errorf("%s has %d entries, want %d: %v", key, len(rows), c.n, rows)
			continue
		}
		first := rows[0]
		if c.id != "" {
			for _, r := range rows {
				if r["id"] == c.id {
					first = r
				}
			}
		}
		if got := members(t, first); !slices.Equal(got, c.members) {
			t.Errorf("%s members %v, want %v", key, got, c.members)
		}
		if c.id != "" && first["id"] != c.id {
			t.Errorf("%s misses the row %s: %v", key, c.id, rows)
		}
	}
	// The values the members carry.
	if c := list(t, exportSection(t, out, personaldata.FileIdentities, "local_credentials"))[0]; c["totp_enrolled"] != true || c["identity_id"] != a.identity.String() {
		t.Errorf("local credential %v", c)
	}
	for _, r := range list(t, exportSection(t, out, personaldata.FileDevices, "sessions")) {
		if r["id"] == a.session.String() && r["last_ip"] != "192.0.2.10" {
			t.Errorf("session %v", r)
		}
	}
	for _, r := range list(t, exportSection(t, out, personaldata.FileAccess, "owner")) {
		if r["role"] != "owner" {
			t.Errorf("owner entry %v", r)
		}
	}

	// Nothing of Sam, no secret, hash, key or counter in any section.
	var all []byte
	for _, f := range []personaldata.File{personaldata.FileProfile, personaldata.FileIdentities, personaldata.FileDevices, personaldata.FileAccess} {
		for _, raw := range out.Sections[f] {
			all = append(all, raw...)
		}
	}
	for _, bad := range []string{
		sam.ID.String(), s.device.String(), s.session.String(), s.token.String(), s.passkey.String(), s.identity.String(), "sam@example.com",
		"argon2id", "password_hash", "token_hash", "token_prefix", "bothy_pat", "public_key", "credential_id", "sign_count",
		"totp_secret", "jkt", "cookie_hash", "code_hash", `"sam"`,
	} {
		if bytes.Contains(all, []byte(bad)) {
			t.Errorf("the export holds %q:\n%s", bad, all)
		}
	}
	if len(out.Lines) != 0 || len(out.Contents) != 0 {
		t.Errorf("identity wrote lines or contents: %v %v", out.Lines, out.Contents)
	}

	// An unknown person is not_found.
	err := w.pd.Export(context.Background(), personaldata.Export{Subject: ids.New()}, personaldata.NewMemoryWriter())
	wantCode(t, err, errs.CodeNotFound)
}

// TestPersonalDataExportEmpty: a person without any row of the lists gets every section as an
// empty array, never null.
func TestPersonalDataExportEmpty(t *testing.T) {
	t.Parallel()
	w := newPersonalWorld(t)
	robin := w.person("robin", "active", false)
	out := personaldata.NewMemoryWriter()
	if err := w.pd.Export(context.Background(), personaldata.Export{Subject: robin.ID}, out); err != nil {
		t.Fatal(err)
	}
	for f, keys := range map[personaldata.File][]string{
		personaldata.FileIdentities: {"identities", "passkeys", "local_credentials"},
		personaldata.FileDevices:    {"devices", "sessions", "api_tokens"},
		personaldata.FileAccess:     {"owner"},
	} {
		for _, k := range keys {
			if got := string(out.Sections[f][k]); got != "[]" {
				t.Errorf("%s of %s = %s, want []", k, f, got)
			}
		}
	}
}

// identityCounts returns the number of rows per identity table of the person id (sessions and
// devices included), the evidence of TestPersonalDataErase.
func (w *personalWorld) identityCounts(id ids.ID) map[string]int {
	w.t.Helper()
	out := map[string]int{}
	for name, q := range map[string]string{ // #nosec G101 -- table names and SQL, no credentials
		"user_identities":   `SELECT count(*) FROM user_identities WHERE user_id = $1`,
		"local_credentials": `SELECT count(*) FROM local_credentials c JOIN user_identities i ON i.id = c.identity_id WHERE i.user_id = $1`,
		"passkeys":          `SELECT count(*) FROM passkey_credentials WHERE user_id = $1`,
		"devices":           `SELECT count(*) FROM devices WHERE user_id = $1`,
		"device_keys":       `SELECT count(*) FROM device_keys k JOIN devices d ON d.id = k.device_id WHERE d.user_id = $1`,
		"sessions":          `SELECT count(*) FROM auth_sessions WHERE user_id = $1`,
		"families":          `SELECT count(*) FROM refresh_token_families f JOIN auth_sessions s ON s.id = f.session_id WHERE s.user_id = $1`,
		"refresh_tokens": `SELECT count(*) FROM refresh_tokens t JOIN refresh_token_families f ON f.id = t.family_id
			JOIN auth_sessions s ON s.id = f.session_id WHERE s.user_id = $1`,
		"api_tokens":       `SELECT count(*) FROM api_tokens WHERE principal_id = $1`,
		"enrollment_codes": `SELECT count(*) FROM enrollment_codes WHERE user_id = $1`,
		"login_flows":      `SELECT count(*) FROM auth_login_flows WHERE user_id = $1`,
		"pairings":         `SELECT count(*) FROM device_pairings WHERE user_id = $1`,
		"challenges": `SELECT count(*) FROM auth_challenges c WHERE c.session_id IN (SELECT id FROM auth_sessions WHERE user_id = $1)
			OR c.enrollment_code_id IN (SELECT id FROM enrollment_codes WHERE user_id = $1)`,
		"owners": `SELECT count(*) FROM owners WHERE user_id = $1`,
	} {
		out[name] = w.count(q, id.String())
	}
	return out
}

// TestPersonalDataErase (M7-06, 20 section 4.4): every step 1 to 6 runs twice in order (the
// second run is the resume after a crash) in both content modes. Step 1 disables Alex as the
// system, ends sessions, devices and tokens and revokes Alex's owner role; step 2 deletes every
// security row of Alex, and the device and session a terminal session holds stay with their
// personal columns cleared; step 6 pseudonymizes principals and users and releases the avatar.
// Sam's rows stay as they were, and the repeat writes no second event.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			w := newPersonalWorld(t)
			w.person("owner", "active", true)
			alex := w.person("alex", "active", false)
			sam := w.person("sam", "active", false)
			a := w.rows(alex.ID, "alex", true)
			w.rows(sam.ID, "sam", false)
			samBefore := w.identityCounts(sam.ID)
			ctx := context.Background()
			e := personaldata.Erasure{Subject: alex.ID, RequestID: ids.New(), Content: mode}
			userUpdated := func() int {
				return w.count(`SELECT count(*) FROM event_log WHERE type = 'user.updated' AND resource_id = $1 AND actor_kind = 'system'
					AND actor_principal_id IS NULL`, alex.ID.String())
			}

			for run := 1; run <= 2; run++ {
				for _, step := range personaldata.ParticipantSteps() {
					if err := w.pd.Erase(ctx, step, e); err != nil {
						t.Fatalf("run %d step %s: %v", run, step, err)
					}
					if run == 1 && step == personaldata.StepDisable {
						if s := w.text(`SELECT status FROM principals WHERE id = $1`, alex.ID.String()); s != "disabled" {
							t.Errorf("after step 1 Alex is %s", s)
						}
						if n := w.count(`SELECT count(*) FROM auth_sessions WHERE user_id = $1 AND revoked_at IS NULL`, alex.ID.String()); n != 0 {
							t.Errorf("after step 1 Alex has %d live sessions", n)
						}
						if n := w.count(`SELECT count(*) FROM devices WHERE user_id = $1 AND revoke_reason = 'user_disabled' AND revoked_by IS NULL`, alex.ID.String()); n != 2 {
							t.Errorf("after step 1 %d devices are revoked by the system", n)
						}
						if n := w.count(`SELECT count(*) FROM owners WHERE user_id = $1 AND revoked_at IS NULL`, alex.ID.String()); n != 0 {
							t.Errorf("after step 1 Alex holds %d owner roles", n)
						}
						if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'session.revoked'`); n != 3 {
							t.Errorf("after step 1 %d session.revoked events", n)
						}
						if n := userUpdated(); n != 1 {
							t.Errorf("after step 1 %d user.updated events of the system", n)
						}
					}
				}
			}

			// Step 2: only the held device and session stay, cleared.
			got := w.identityCounts(alex.ID)
			for name, want := range map[string]int{
				"user_identities": 0, "local_credentials": 0, "passkeys": 0, "devices": 1, "device_keys": 0, "sessions": 1,
				"families": 0, "refresh_tokens": 0, "api_tokens": 0, "enrollment_codes": 0, "login_flows": 0, "pairings": 0,
				"challenges": 0, "owners": 2,
			} {
				if got[name] != want {
					t.Errorf("Alex has %d %s, want %d", got[name], name, want)
				}
			}
			if s := w.text(`SELECT id::text FROM devices WHERE user_id = $1`, alex.ID.String()); s != a.heldDevice.String() {
				t.Errorf("the kept device is %s, want the held one", s)
			}
			if s := w.text(`SELECT name || '/' || COALESCE(app_version, '') FROM devices WHERE id = $1`, a.heldDevice.String()); s != "Deleted device/" {
				t.Errorf("the held device is %q", s)
			}
			if s := w.text(`SELECT COALESCE(host(last_ip), '') FROM auth_sessions WHERE id = $1`, a.heldSession.String()); s != "" {
				t.Errorf("the held session keeps the IP %s", s)
			}

			// Step 6: no clear data in principals and users.
			if s := w.text(`SELECT handle FROM principals WHERE id = $1`, alex.ID.String()); !strings.HasPrefix(s, "deleted-") || len(s) != len("deleted-")+8 {
				t.Errorf("handle %q", s)
			}
			if s := w.text(`SELECT display_name FROM principals WHERE id = $1`, alex.ID.String()); !strings.HasPrefix(s, "Deleted person ") {
				t.Errorf("display name %q", s)
			}
			if s := w.text(`SELECT concat_ws('/', status, avatar_file_id::text, (deleted_at IS NOT NULL)::text, (disabled_at IS NOT NULL)::text)
				FROM principals WHERE id = $1`, alex.ID.String()); s != "deleted/true/true" {
				t.Errorf("principal state %q", s)
			}
			if s := w.text(`SELECT concat_ws('/', email, locale, time_zone, last_active_at::text) FROM users WHERE principal_id = $1`, alex.ID.String()); s != "de/UTC" {
				t.Errorf("users row %q", s)
			}
			if !slices.Equal(w.files.released, []ids.ID{a.avatar}) {
				t.Errorf("released avatars %v, want %v once", w.files.released, a.avatar)
			}
			if n := userUpdated(); n != 2 {
				t.Errorf("%d user.updated events of the system, want one per changing step", n)
			}
			// The pseudonym is in the event, never the old name.
			if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'user.updated' AND resource_id = $1
				AND payload->'user'->>'status' = 'deleted' AND payload->'user'->>'handle' LIKE 'deleted-%'`, alex.ID.String()); n != 1 {
				t.Errorf("%d user.updated events carry the pseudonym", n)
			}

			// Sam is untouched.
			if after := w.identityCounts(sam.ID); !mapsEqual(after, samBefore) {
				t.Errorf("Sam's rows changed: %v, before %v", after, samBefore)
			}
			if s := w.text(`SELECT concat_ws('/', p.status, p.handle, u.email) FROM principals p JOIN users u ON u.principal_id = p.id WHERE p.id = $1`,
				sam.ID.String()); s != "active/sam/sam@example.com" {
				t.Errorf("Sam is %q", s)
			}
		})
	}
}

// mapsEqual reports equal count maps.
func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// TestPersonalDataEraseLastOwner (20 section 4.4 item 1, 07 SR8): the deletion of the last active
// owner stops at step 1 with last_owner and changes nothing; a disabled owner does not count.
func TestPersonalDataEraseLastOwner(t *testing.T) {
	t.Parallel()
	w := newPersonalWorld(t)
	o := w.person("owner", "active", true)
	w.person("former", "disabled", true)
	err := w.pd.Erase(context.Background(), personaldata.StepDisable, personaldata.Erasure{Subject: o.ID, Content: personaldata.KeepPseudonymized})
	wantCode(t, err, errs.CodeLastOwner)
	if s := w.text(`SELECT status FROM principals WHERE id = $1`, o.ID.String()); s != "active" {
		t.Errorf("the last owner is %s", s)
	}
	if n := w.count(`SELECT count(*) FROM owners WHERE user_id = $1 AND revoked_at IS NULL`, o.ID.String()); n != 1 {
		t.Errorf("the last owner holds %d owner roles", n)
	}
	// Steps without work for identity and an unknown person.
	if err := w.pd.Erase(context.Background(), personaldata.StepContent, personaldata.Erasure{Subject: o.ID}); err != nil {
		t.Errorf("step content = %v", err)
	}
	wantCode(t, w.pd.Erase(context.Background(), personaldata.StepDisable, personaldata.Erasure{Subject: ids.New()}), errs.CodeNotFound)
}

// TestNewPersonalData_Refuses: the participant needs its collaborators and names its module and
// the identity tables of 20 section 2.
func TestNewPersonalData_Refuses(t *testing.T) {
	t.Parallel()
	if _, err := app.NewPersonalData(nil, nil); err == nil {
		t.Error("NewPersonalData without collaborators passed")
	}
	w := newPersonalWorld(t)
	if w.pd.Module() != "identity" || len(w.pd.Tables()) != 16 {
		t.Errorf("module %s, tables %v", w.pd.Module(), w.pd.Tables())
	}
	reg := personaldata.NewRegistry()
	if err := reg.Register(w.pd); err != nil {
		t.Error(err)
	}
}
