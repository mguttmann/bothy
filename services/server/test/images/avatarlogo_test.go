// SPDX-License-Identifier: Apache-2.0

package images_test

// The integration tests of brief M3-10 (avatar and logo) on the server of this package: Alex and
// Sam are people of the matrix world, Lotse an agent, the harness owner owns the instance. The
// images are the synthetic fixtures of build_test.go. TestAvatarLogoScanner runs the cases that
// need a scanner verdict against the scripted clamd of M3-04 (clamdtest.Fake).

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image/jpeg"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// avatarEnv is env with the owner, the agent Lotse and the answer of getBrandingLogo while no logo
// exists.
type avatarEnv struct {
	*env
	owner  *harness.Client
	lotse  *harness.Client
	noLogo harness.Response
}

// startAvatarEnv starts the server of the package and builds the matrix world on it.
func startAvatarEnv(t *testing.T) *avatarEnv {
	t.Helper()
	h := &hooks{}
	logs := &safeBuffer{}
	s := harness.Start(t, harness.Options{Config: config(), LogWriter: logs, FilesImageHooks: h.seams()})
	w := harness.BuildMatrixWorld(t, s)
	e := &env{s: s, alex: w.People["Alex"], sam: w.People["Sam"], channel: w.Channels["Support/Allgemein"], db: s.AdminDB(), logs: logs, hooks: h}
	return &avatarEnv{env: e, owner: s.Owner, lotse: w.Agents["Lotse"]}
}

// TestAvatarAndLogo runs the cases of brief M3-10 that need no scanner verdict on one server, one
// after another.
func TestAvatarAndLogo(t *testing.T) {
	e := startAvatarEnv(t)
	t.Run("GetBrandingLogoWithoutLogo", func(t *testing.T) { testNoLogo(t, e) })
	t.Run("SetMyAvatarHappyPath", func(t *testing.T) { testSetMyAvatarHappyPath(t, e) })
	t.Run("SetMyAvatarReplacesPrevious", func(t *testing.T) { testSetMyAvatarReplacesPrevious(t, e) })
	t.Run("SetMyAvatarRejections", func(t *testing.T) { testSetMyAvatarRejections(t, e) })
	t.Run("SetMyAvatarAuthorizeBeforeBody", func(t *testing.T) { testSetMyAvatarAuthorizeBeforeBody(t, e) })
	t.Run("SetMyAvatarDeclaredTypeLies", func(t *testing.T) { testDeclaredTypeLies(t, e) })
	t.Run("SetMyAvatarAnimatedWebPFails", func(t *testing.T) { testAnimatedWebPFails(t, e) })
	t.Run("SetMyAvatarGIFAsPNGFails", func(t *testing.T) { testGIFAsPNGFails(t, e) })
	t.Run("AvatarReaders", func(t *testing.T) { testAvatarReaders(t, e) })
	t.Run("DeleteMyAvatar", func(t *testing.T) { testDeleteMyAvatar(t, e) })
	t.Run("AvatarConcurrentReplace", func(t *testing.T) { testAvatarConcurrentReplace(t, e) })
	t.Run("UploadBrandingLogoHappyPath", func(t *testing.T) { testUploadLogoHappyPath(t, e) })
	t.Run("UploadBrandingLogoRights", func(t *testing.T) { testUploadLogoRights(t, e) })
	t.Run("LogoPublicOnlyThroughGetBrandingLogo", func(t *testing.T) { testLogoPublicOnly(t, e) })
	t.Run("GetBrandingLogoOverPixelLimit", func(t *testing.T) { testLogoOverPixelLimit(t, e) })
	t.Run("DeleteBrandingLogo", func(t *testing.T) { testDeleteBrandingLogo(t, e) })
	t.Run("LogoVersionMonotonic", func(t *testing.T) { testLogoVersionMonotonic(t, e) })
	t.Run("ImagesWithoutMetadata", func(t *testing.T) { testImagesWithoutMetadata(t, e) })
	t.Run("ImageUploadSEC_T08", func(t *testing.T) { testImageUploadSECT08(t, e) })
	// Last: it checks the events of every avatar and logo file of the cases above.
	t.Run("AvatarAndLogoFileEvents", func(t *testing.T) { testAvatarAndLogoFileEvents(t, e) })
}

// getLogo calls getBrandingLogo without credentials.
func (e *avatarEnv) getLogo(t *testing.T) harness.Response {
	t.Helper()
	return e.owner.DoAnonymous(t.Context(), harness.Request{Method: http.MethodGet, Path: pathLogo})
}

// testNoLogo: before any logo the answer is 404, the baseline of every later 404 of the route.
func testNoLogo(t *testing.T, e *avatarEnv) {
	e.noLogo = e.getLogo(t)
	if e.noLogo.Status != http.StatusNotFound || e.noLogo.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("getBrandingLogo without logo: %d %v %s", e.noLogo.Status, e.noLogo.Header, e.noLogo.Body)
	}
}

// setAvatar uploads data with the declared type as c and returns the User of a 200.
func setAvatar(t *testing.T, c *harness.Client, ctype string, data []byte) user {
	t.Helper()
	r := putImage(t, c, pathAvatar, file(ctype, data))
	if r.Status != http.StatusOK {
		t.Fatalf("setMyAvatar: %d %s", r.Status, r.Body)
	}
	var u user
	r.JSON(t, &u)
	if u.AvatarFileID == "" || u.ID != c.PrincipalID {
		t.Fatalf("setMyAvatar: %s", r.Body)
	}
	return u
}

// testSetMyAvatarHappyPath: 200 with avatar_file_id, the row of purpose avatar without channel
// with the fixed name, the completed session, one files.scan job, one user.updated, and after the
// jobs a deliverable file with the variants of M3-06.
func testSetMyAvatarHappyPath(t *testing.T, e *avatarEnv) {
	in, _ := metadataJPEG(t)
	before := len(e.eventsOf(t, "user.updated", e.alex.PrincipalID))
	u := setAvatar(t, e.alex, mimeJPEG, in)
	id := u.AvatarFileID
	var purpose, name, by, state string
	var channel *string
	if err := e.db.QueryRow(t.Context(), `SELECT f.purpose, f.original_name, f.uploaded_by::text, f.scope_channel_id::text, s.state
		FROM files f JOIN file_upload_sessions s ON s.file_id = f.id WHERE f.id = $1`, id).Scan(&purpose, &name, &by, &channel, &state); err != nil {
		t.Fatal(err)
	}
	if purpose != "avatar" || name != "avatar" || by != e.alex.PrincipalID || channel != nil || state != "completed" {
		t.Fatalf("row: %s %s %s %v %s", purpose, name, by, channel, state)
	}
	if n := e.rowCount(t, `SELECT count(*) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1`, id); n != 1 {
		t.Errorf("%d files.scan jobs", n)
	}
	evs := e.eventsOf(t, "user.updated", e.alex.PrincipalID)
	if len(evs) != before+1 || !strings.Contains(string(evs[len(evs)-1].Payload), id) {
		t.Errorf("user.updated: %d new, last %s", len(evs)-before, evs[len(evs)-1].Payload)
	}
	if e.avatarOf(t, e.alex.PrincipalID) != id {
		t.Error("principals.avatar_file_id not set")
	}
	r := e.waitJobDone(t, id)
	if r.UploadState != stateComplete || r.ScanStatus != scanSkipped || e.thumbnails(t, id) != 3 {
		t.Fatalf("after the jobs: %+v, %d thumbnails", r, e.thumbnails(t, id))
	}
}

// testSetMyAvatarReplacesPrevious: a second avatar marks the first deleted, and getFile on it
// answers exactly like an unknown id (AZ-P4).
func testSetMyAvatarReplacesPrevious(t *testing.T, e *avatarEnv) {
	first := setAvatar(t, e.alex, mimePNG, encodePNG(t, twoColour(8, 8, colourFirst, colourSecond))).AvatarFileID
	second := setAvatar(t, e.alex, mimePNG, encodePNG(t, twoColour(8, 8, colourSecond, colourFirst))).AvatarFileID
	if !e.readFile(t, first).Deleted || e.readFile(t, second).Deleted || e.avatarOf(t, e.alex.PrincipalID) != second {
		t.Fatal("the previous avatar is not released")
	}
	gone := e.sam.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + first})
	unknown := e.sam.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + "0192f4a0-7b1c-7f89-8d6b-6960453241bc"})
	if gone.Status != http.StatusNotFound {
		t.Fatalf("getFile of a replaced avatar: %d", gone.Status)
	}
	sameAnswer(t, "replaced avatar and unknown id", gone, unknown)
}

// testSetMyAvatarRejections: the checks of design note 2 b to d; nothing is stored after any.
func testSetMyAvatarRejections(t *testing.T, e *avatarEnv) {
	png := encodePNG(t, twoColour(4, 4, colourFirst, colourSecond))
	filesBefore := e.rowCount(t, `SELECT count(*) FROM files`)
	sessionsBefore := e.rowCount(t, `SELECT count(*) FROM file_upload_sessions`)
	for _, c := range []struct {
		name    string
		parts   []part
		status  int
		pointer string
		detail  string
	}{
		{"gif", []part{file("image/gif", metadataGIF(t))}, http.StatusUnsupportedMediaType, "", ""},
		{"svg", []part{file("image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))},
			http.StatusUnsupportedMediaType, "", ""},
		{"one byte over the cap", []part{file(mimePNG, bytes.Repeat([]byte{1}, partBytes+1))}, http.StatusRequestEntityTooLarge, "",
			"file.upload_part_bytes"},
		{"no part file", []part{{name: "image", ctype: mimePNG, data: png}}, http.StatusBadRequest, "/file", ""},
		{"empty part", []part{file(mimePNG, nil)}, http.StatusBadRequest, "/file", ""},
		{"second part", []part{file(mimePNG, png), {name: "extra", ctype: "text/plain", data: []byte("x")}}, http.StatusBadRequest, "/extra", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := putImage(t, e.alex, pathAvatar, c.parts...)
			p := problemOf(t, r)
			if r.Status != c.status || !strings.Contains(p.Detail, c.detail) ||
				(c.pointer != "" && (len(p.Errors) != 1 || p.Errors[0].Pointer != c.pointer)) {
				t.Fatalf("%d %s", r.Status, r.Body)
			}
			if c.status == http.StatusUnsupportedMediaType && p.Reason != "" {
				t.Errorf("reason %q for a type outside ImageUpload", p.Reason)
			}
		})
	}
	if e.rowCount(t, `SELECT count(*) FROM files`) != filesBefore || e.rowCount(t, `SELECT count(*) FROM file_upload_sessions`) != sessionsBefore {
		t.Error("a refused image left a row")
	}
}

// testSetMyAvatarAuthorizeBeforeBody: anonymous gets 401 and an agent 403
// subject_kind_not_allowed while the body is held back, so authorize ran before the body; nothing
// is stored.
func testSetMyAvatarAuthorizeBeforeBody(t *testing.T, e *avatarEnv) {
	filesBefore := e.rowCount(t, `SELECT count(*) FROM files`)
	if r := putUnread(t, e.alex, pathAvatar, true); r.Status != http.StatusUnauthorized {
		t.Errorf("anonymous: %d %s", r.Status, r.Body)
	}
	r := putUnread(t, e.lotse, pathAvatar, false)
	if r.Status != http.StatusForbidden || problemOf(t, r).Reason != "subject_kind_not_allowed" {
		t.Errorf("agent: %d %s", r.Status, r.Body)
	}
	if e.rowCount(t, `SELECT count(*) FROM files`) != filesBefore {
		t.Error("a refused caller stored a file")
	}
}

// failedAvatar uploads data declared as ctype as Alex and waits until the file is final: it must
// end failed with file_type_denied and never be deliverable.
func failedAvatar(t *testing.T, e *avatarEnv, ctype string, data []byte) {
	t.Helper()
	id := setAvatar(t, e.alex, ctype, data).AvatarFileID
	var r fileRow
	poll(t, "the refusal of "+id, func() bool {
		r = e.readFile(t, id)
		return r.UploadState == stateFailed
	})
	if r.FailureReason == nil || *r.FailureReason != "file_type_denied" {
		t.Fatalf("row %+v", r)
	}
	status, f := getFile(t, e.alex, id)
	if status != http.StatusOK || f.UploadState != stateFailed {
		t.Errorf("getFile: %d %+v", status, f)
	}
	if objs := e.objects(t, id); len(objs) != 0 {
		t.Errorf("objects of a refused avatar: %v", objs)
	}
}

// testDeclaredTypeLies: declared PNG, the bytes are HTML: failed with file_type_denied after the
// scan pass.
func testDeclaredTypeLies(t *testing.T, e *avatarEnv) {
	failedAvatar(t, e, mimePNG, []byte("<!DOCTYPE html><html><body>canary page of Alex at example.com</body></html>"))
}

// testAnimatedWebPFails: an animated WebP is stored unchanged by files.thumbnail, which the purpose
// rule refuses.
func testAnimatedWebPFails(t *testing.T, e *avatarEnv) {
	failedAvatar(t, e, mimeWebP, animatedWebP(t))
}

// testGIFAsPNGFails: a GIF declared as PNG is refused by the purpose rule of the scan pass.
func testGIFAsPNGFails(t *testing.T, e *avatarEnv) {
	failedAvatar(t, e, mimePNG, metadataGIF(t))
}

// testAvatarReaders: Sam and the agent Lotse read Alex's avatar with getFile; anonymous gets 401.
func testAvatarReaders(t *testing.T, e *avatarEnv) {
	id := setAvatar(t, e.alex, mimePNG, encodePNG(t, twoColour(8, 8, colourFirst, colourSecond))).AvatarFileID
	for name, c := range map[string]*harness.Client{"Sam": e.sam, "Lotse": e.lotse} {
		r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + id})
		var f struct {
			Purpose string `json:"purpose"`
		}
		if r.Status == http.StatusOK {
			r.JSON(t, &f)
		}
		if r.Status != http.StatusOK || f.Purpose != "avatar" {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	if r := e.sam.DoAnonymous(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + id}); r.Status != http.StatusUnauthorized {
		t.Errorf("anonymous: %d", r.Status)
	}
}

// testDeleteMyAvatar: 204, the link cleared, the file released, one user.updated; a second call
// 204 without event; no audit entry for the avatar operations.
func testDeleteMyAvatar(t *testing.T, e *avatarEnv) {
	id := setAvatar(t, e.alex, mimePNG, encodePNG(t, twoColour(8, 8, colourFirst, colourSecond))).AvatarFileID
	before := len(e.eventsOf(t, "user.updated", e.alex.PrincipalID))
	del := func() int {
		return e.alex.Do(t.Context(), harness.Request{Method: http.MethodDelete, Path: pathAvatar}).Status
	}
	if s := del(); s != http.StatusNoContent {
		t.Fatalf("deleteMyAvatar: %d", s)
	}
	if e.avatarOf(t, e.alex.PrincipalID) != "" || !e.readFile(t, id).Deleted {
		t.Fatal("the avatar is still linked or not released")
	}
	if n := len(e.eventsOf(t, "user.updated", e.alex.PrincipalID)); n != before+1 {
		t.Fatalf("%d user.updated", n-before)
	}
	if s := del(); s != http.StatusNoContent {
		t.Fatalf("second deleteMyAvatar: %d", s)
	}
	if n := len(e.eventsOf(t, "user.updated", e.alex.PrincipalID)); n != before+1 {
		t.Errorf("the second call wrote %d events", n-before-1)
	}
	var me struct {
		Principal map[string]any `json:"principal"`
	}
	harness.MustDo(t, e.alex, harness.Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK).JSON(t, &me)
	if _, ok := me.Principal["avatar_file_id"]; ok {
		t.Errorf("getMe still names an avatar: %v", me.Principal)
	}
	if n := e.rowCount(t, `SELECT count(*) FROM audit_events WHERE action = 'self.profile.update'`); n != 0 {
		t.Errorf("%d audit entries of self.profile.update", n)
	}
}

// testAvatarConcurrentReplace: two parallel uploads end 200 (no deadlock reaches the client);
// afterwards exactly one file is linked and every other avatar of Alex is deleted.
func testAvatarConcurrentReplace(t *testing.T, e *avatarEnv) {
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i := range statuses {
		wg.Go(func() {
			statuses[i] = putImage(t, e.alex, pathAvatar, file(mimePNG, encodePNG(t, twoColour(8, 8+i, colourFirst, colourSecond)))).Status
		})
	}
	wg.Wait()
	for _, s := range statuses {
		if s != http.StatusOK {
			t.Fatalf("statuses %v", statuses)
		}
	}
	linked := e.avatarOf(t, e.alex.PrincipalID)
	if n := e.rowCount(t, `SELECT count(*) FROM files WHERE uploaded_by = $1 AND purpose = 'avatar' AND deleted_at IS NULL
		AND upload_state <> 'failed'`, e.alex.PrincipalID); n != 1 || linked == "" || e.readFile(t, linked).Deleted {
		t.Fatalf("%d live avatars, linked %q", n, linked)
	}
}

// uploadLogo uploads data as the owner and returns the Branding of a 200.
func (e *avatarEnv) uploadLogo(t *testing.T, ctype string, data []byte) branding {
	t.Helper()
	r := putImage(t, e.owner, pathLogo, file(ctype, data))
	if r.Status != http.StatusOK {
		t.Fatalf("uploadBrandingLogo: %d %s", r.Status, r.Body)
	}
	var b branding
	r.JSON(t, &b)
	return b
}

// currentBranding reads the branding from getInstance.
func (e *avatarEnv) currentBranding(t *testing.T) branding {
	t.Helper()
	var info struct {
		Branding branding `json:"branding"`
	}
	harness.MustDo(t, e.sam, harness.Request{Method: http.MethodGet, Path: "/instance"}, http.StatusOK).JSON(t, &info)
	return info.Branding
}

// auditRows returns the details of the audit entries of instance.branding.update with decision.
func (e *avatarEnv) auditRows(t *testing.T, decision string) []string {
	t.Helper()
	rows, err := e.db.Query(t.Context(), `SELECT coalesce(details::text, '') FROM audit_events WHERE action = 'instance.branding.update' AND decision = $1
		ORDER BY seq`, decision)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

// checkLogoResponse checks the headers of note 8 and that the body is the variant medium of id.
func checkLogoResponse(t *testing.T, e *avatarEnv, r harness.Response, id string) {
	t.Helper()
	medium := e.thumbRows(t, id)[1]
	want := e.object(t, medium.ObjectKey)
	sum := sha256.Sum256(want)
	ext := map[string]string{mimeJPEG: "jpg", mimePNG: "png"}[medium.MIMEType]
	h := r.Header
	for k, v := range map[string]string{
		"Content-Type": medium.MIMEType, "Content-Disposition": `inline; filename="logo.` + ext + `"; filename*=UTF-8''logo.` + ext,
		"Content-Security-Policy": "default-src 'none'; sandbox", "X-Content-Type-Options": "nosniff",
		"Cross-Origin-Resource-Policy": "same-origin", "Cache-Control": "no-store", "ETag": `"` + hex.EncodeToString(sum[:]) + `"`,
		"Repr-Digest": "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":", "Access-Control-Allow-Origin": "*",
		"Content-Length": strconv.Itoa(len(want)),
	} {
		if h.Get(k) != v {
			t.Errorf("%s: %q, want %q", k, h.Get(k), v)
		}
	}
	if h.Get("Access-Control-Allow-Credentials") != "" {
		t.Error("Access-Control-Allow-Credentials on the public logo")
	}
	if r.Status != http.StatusOK || !bytes.Equal(r.Body, want) {
		t.Fatalf("getBrandingLogo: %d, %d bytes, want the %d bytes of the variant medium", r.Status, len(r.Body), len(want))
	}
}

// testUploadLogoHappyPath: 200 with logo_file_id and the version raised by one, one
// instance.updated naming only the logo key, one audit allow entry with changed_keys and file_id,
// and after the jobs the variant medium for anonymous callers with the headers of note 8.
func testUploadLogoHappyPath(t *testing.T, e *avatarEnv) {
	in, _ := metadataPNG(t)
	before := e.currentBranding(t)
	instanceID := e.instanceID(t)
	events := len(e.eventsOf(t, "instance.updated", instanceID))
	allows := len(e.auditRows(t, "allow"))
	b := e.uploadLogo(t, mimePNG, in)
	if b.LogoFileID == "" || b.Version != before.Version+1 {
		t.Fatalf("branding %+v after %+v", b, before)
	}
	evs := e.eventsOf(t, "instance.updated", instanceID)
	if len(evs) != events+1 || !jsonEqual(t, evs[len(evs)-1].Payload, `{"changed_keys":["branding.logo_file_id"]}`) {
		t.Fatalf("instance.updated: %d new, last %s", len(evs)-events, evs[len(evs)-1].Payload)
	}
	audits := e.auditRows(t, "allow")
	if len(audits) != allows+1 || !strings.Contains(audits[len(audits)-1], b.LogoFileID) ||
		!strings.Contains(audits[len(audits)-1], "branding.logo_file_id") {
		t.Fatalf("audit allow entries: %v", audits[allows:])
	}
	e.waitJobDone(t, b.LogoFileID)
	checkLogoResponse(t, e, e.getLogo(t), b.LogoFileID)
}

// instanceID reads the instance id.
func (e *avatarEnv) instanceID(t *testing.T) string {
	t.Helper()
	var id string
	if err := e.db.QueryRow(t.Context(), `SELECT instance_id::text FROM instance`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// jsonEqual reports whether a and the JSON text b are the same value.
func jsonEqual(t *testing.T, a []byte, b string) bool {
	t.Helper()
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return false
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return bytes.Equal(xa, ya)
}

// testUploadLogoRights: Sam 403 owner_only with one deny entry, an owner session without step-up
// 401 step_up_required, the owner with a personal API token 403 owner_only, anonymous 401; the
// body is never read.
func testUploadLogoRights(t *testing.T, e *avatarEnv) {
	denies := len(e.auditRows(t, "deny"))
	r := putUnread(t, e.sam, pathLogo, false)
	if r.Status != http.StatusForbidden || problemOf(t, r).Reason != "owner_only" {
		t.Errorf("Sam: %d %s", r.Status, r.Body)
	}
	poll(t, "the deny entry of Sam", func() bool { return len(e.auditRows(t, "deny")) > denies })
	if n := len(e.auditRows(t, "deny")); n != denies+1 {
		t.Errorf("%d deny entries for Sam", n-denies)
	}
	fresh, err := e.s.OwnerSession(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if r := putUnread(t, fresh, pathLogo, false); r.Status != http.StatusUnauthorized || problemOf(t, r).Code != "step_up_required" {
		t.Errorf("owner without step-up: %d %s", r.Status, r.Body)
	}
	harness.MustDo(t, e.owner, harness.Request{Method: http.MethodPut, Path: "/feature-flags/tokens.personal_enabled",
		Body: map[string]any{"enabled": true, "reason": "personal token of the owner"}}, http.StatusOK)
	token := e.s.NewTokenClient(t, e.owner)
	// A personal token of an owner acts without owner rights (07 SR5), so the requirement owner
	// refuses it before the step-up rule (docs/decisions.d/M3-10.md D-M3-10-19).
	if r := putUnread(t, token, pathLogo, false); r.Status != http.StatusForbidden || problemOf(t, r).Reason != "owner_only" {
		t.Errorf("owner with a personal token: %d %s", r.Status, r.Body)
	}
	if r := putUnread(t, e.sam, pathLogo, true); r.Status != http.StatusUnauthorized {
		t.Errorf("anonymous: %d %s", r.Status, r.Body)
	}
}

// testLogoPublicOnly: the logo file is not public on the file routes; signed in, Sam reads its
// metadata.
func testLogoPublicOnly(t *testing.T, e *avatarEnv) {
	id := e.currentBranding(t).LogoFileID
	if id == "" {
		t.Fatal("no logo")
	}
	if r := e.sam.DoAnonymous(t.Context(), harness.Request{Method: http.MethodGet, Path: "/files/" + id}); r.Status != http.StatusUnauthorized {
		t.Errorf("anonymous getFile: %d", r.Status)
	}
	r := harness.MustDo(t, e.sam, harness.Request{Method: http.MethodGet, Path: "/files/" + id}, http.StatusOK)
	var f struct {
		Purpose string `json:"purpose"`
	}
	r.JSON(t, &f)
	if f.Purpose != "branding" {
		t.Errorf("purpose %q", f.Purpose)
	}
}

// testLogoOverPixelLimit: a JPEG logo above file.image.max_pixels is deliverable without variants,
// so getBrandingLogo answers the same 404 as without a logo.
func testLogoOverPixelLimit(t *testing.T, e *avatarEnv) {
	b := e.uploadLogo(t, mimeJPEG, hugeJPEG(t))
	r := e.waitJobDone(t, b.LogoFileID)
	if r.UploadState != stateComplete || e.thumbnails(t, b.LogoFileID) != 0 {
		t.Fatalf("row %+v", r)
	}
	sameAnswer(t, "logo without variant and no logo", e.getLogo(t), e.noLogo)
}

// testDeleteBrandingLogo: 204; the empty string with a raised version; no logo in discovery,
// getInstance and getBrandingLogo; the old file released; one event and one audit entry; a second
// call 204 without event and with an audit entry of empty changed_keys.
func testDeleteBrandingLogo(t *testing.T, e *avatarEnv) {
	old := e.uploadLogo(t, mimePNG, encodePNG(t, twoColour(8, 8, colourFirst, colourSecond)))
	instanceID := e.instanceID(t)
	events := len(e.eventsOf(t, "instance.updated", instanceID))
	allows := len(e.auditRows(t, "allow"))
	del := func() int {
		r, err := harness.DoWithStepUp(t.Context(), e.owner, harness.Request{Method: http.MethodDelete, Path: pathLogo})
		if err != nil {
			t.Fatal(err)
		}
		return r.Status
	}
	if s := del(); s != http.StatusNoContent {
		t.Fatalf("deleteBrandingLogo: %d", s)
	}
	var value string
	var version int64
	if err := e.db.QueryRow(t.Context(), `SELECT value #>> '{}', version FROM instance_settings WHERE key = 'branding.logo_file_id'`).
		Scan(&value, &version); err != nil {
		t.Fatal(err)
	}
	now := e.currentBranding(t)
	if value != "" || now.LogoFileID != "" || now.Version != old.Version+1 {
		t.Fatalf("setting %q version %d, branding %+v after %+v", value, version, now, old)
	}
	var disc map[string]any
	e.sam.DoAnonymous(t.Context(), harness.Request{Method: http.MethodGet, Path: "/instance/discovery"}).JSON(t, &disc)
	if br, _ := disc["branding"].(map[string]any); br == nil || br["logo_url"] != nil {
		t.Errorf("discovery %v", disc["branding"])
	}
	sameAnswer(t, "removed logo and no logo", e.getLogo(t), e.noLogo)
	if !e.readFile(t, old.LogoFileID).Deleted {
		t.Error("the old logo is not released")
	}
	if n := len(e.eventsOf(t, "instance.updated", instanceID)); n != events+1 {
		t.Errorf("%d instance.updated", n-events)
	}
	if s := del(); s != http.StatusNoContent {
		t.Fatalf("second deleteBrandingLogo: %d", s)
	}
	if n := len(e.eventsOf(t, "instance.updated", instanceID)); n != events+1 {
		t.Errorf("the second removal wrote an event")
	}
	audits := e.auditRows(t, "allow")
	if len(audits) != allows+2 || !strings.Contains(audits[allows], "branding.logo_file_id") ||
		strings.Contains(audits[allows+1], "branding.logo_file_id") {
		t.Errorf("audit allow entries %v", audits[allows:])
	}
}

// testLogoVersionMonotonic: upload, remove, upload raise the branding version strictly; an old
// If-Match of updateBranding is 412, and updateBranding keeps the logo.
func testLogoVersionMonotonic(t *testing.T, e *avatarEnv) {
	first := e.currentBranding(t)
	a := e.uploadLogo(t, mimePNG, encodePNG(t, twoColour(8, 8, colourFirst, colourSecond)))
	if r, err := harness.DoWithStepUp(t.Context(), e.owner, harness.Request{Method: http.MethodDelete, Path: pathLogo}); err != nil ||
		r.Status != http.StatusNoContent {
		t.Fatalf("remove: %v %d", err, r.Status)
	}
	removed := e.currentBranding(t)
	b := e.uploadLogo(t, mimePNG, encodePNG(t, twoColour(8, 8, colourSecond, colourFirst)))
	if first.Version >= a.Version || a.Version >= removed.Version || removed.Version >= b.Version {
		t.Fatalf("versions %d %d %d %d", first.Version, a.Version, removed.Version, b.Version)
	}
	h := http.Header{}
	h.Set("If-Match", `"`+strconv.FormatInt(first.Version, 10)+`"`)
	body := map[string]any{"product_name": "Team Chat", "accent_color": first.AccentColor}
	r, err := harness.DoWithStepUp(t.Context(), e.owner, harness.Request{Method: http.MethodPut, Path: "/instance/branding", Body: body, Header: h})
	if err != nil || r.Status != http.StatusPreconditionFailed || problemOf(t, r).Code != "version_conflict" {
		t.Fatalf("stale If-Match: %v %d %s", err, r.Status, r.Body)
	}
	h.Set("If-Match", `"`+strconv.FormatInt(b.Version, 10)+`"`)
	r, err = harness.DoWithStepUp(t.Context(), e.owner, harness.Request{Method: http.MethodPut, Path: "/instance/branding", Body: body, Header: h})
	if err != nil || r.Status != http.StatusOK {
		t.Fatalf("updateBranding: %v %d %s", err, r.Status, r.Body)
	}
	var after branding
	r.JSON(t, &after)
	if after.LogoFileID != b.LogoFileID {
		t.Errorf("updateBranding dropped the logo: %+v", after)
	}
}

// testImagesWithoutMetadata (plan: "images re-encoded without metadata"): for avatar and logo the
// stored original of the metadata JPEG, PNG and still WebP is clean, every variant decodes within
// its edge without metadata and with the profile and orientation, and so are the bytes of
// getBrandingLogo.
func testImagesWithoutMetadata(t *testing.T, e *avatarEnv) {
	jpg, jpgProfile := metadataJPEG(t)
	png, pngProfile := metadataPNG(t)
	webp := stillWebP(t)
	for _, target := range []string{"avatar", "logo"} {
		for _, c := range []struct {
			ctype   string
			in      []byte
			profile []byte
		}{{mimeJPEG, jpg, jpgProfile}, {mimePNG, png, pngProfile}, {mimeWebP, webp, nil}} {
			t.Run(target+" "+c.ctype, func(t *testing.T) {
				var id string
				if target == "avatar" {
					id = setAvatar(t, e.alex, c.ctype, c.in).AvatarFileID
				} else {
					id = e.uploadLogo(t, c.ctype, c.in).LogoFileID
				}
				r := e.waitJobDone(t, id)
				if r.UploadState != stateComplete {
					t.Fatalf("row %+v", r)
				}
				stored := e.object(t, r.ObjectKey)
				switch c.ctype {
				case mimeJPEG:
					checkStrippedJPEG(t, c.in, stored, c.profile)
				case mimePNG:
					checkStrippedPNG(t, c.in, stored)
				default:
					if r.MIMEType != mimeJPEG && r.MIMEType != mimePNG {
						t.Fatalf("the WebP is stored as %s", r.MIMEType)
					}
					noMetadata(t, "the converted WebP", stored)
				}
				rows := e.thumbRows(t, id)
				if len(rows) != 3 {
					t.Fatalf("%d variants", len(rows))
				}
				for i, row := range rows {
					b := e.object(t, row.ObjectKey)
					edge := []int{smallPx, mediumPx, largePx}[i]
					if row.Width > edge || row.Height > edge {
						t.Errorf("%s %dx%d above %d", row.Variant, row.Width, row.Height, edge)
					}
					checkVariant(t, c.ctype, row, b, c.profile)
				}
				if target == "logo" {
					checkLogoResponse(t, e, e.getLogo(t), id)
				}
			})
		}
	}
}

// checkVariant checks one variant: it decodes, carries no metadata, and a JPEG variant of the
// JPEG with orientation 6 is portrait with the profile of the original.
func checkVariant(t *testing.T, input string, row thumbRow, b, profile []byte) {
	t.Helper()
	noMetadata(t, row.Variant, b)
	if row.MIMEType != mimeJPEG {
		return
	}
	switch {
	case input == mimeJPEG:
		checkOrientedJPEG(t, row.Variant, b, profile)
	case profile != nil:
		checkVariantJPEG(t, row.Variant, b, profile)
	default:
		if _, err := jpeg.Decode(bytes.NewReader(b)); err != nil {
			t.Errorf("%s: %v", row.Variant, err)
		}
	}
}

// testImageUploadSECT08 (SEC-T08 for these routes, the cases without a scanner verdict): a false
// type declaration fails, SVG with script is 415, an image above file.image.max_pixels gets no
// variant, a size above the cap is 413. The scanner cases run in TestAvatarLogoScanner.
func testImageUploadSECT08(t *testing.T, e *avatarEnv) {
	failedAvatar(t, e, mimeJPEG, []byte("%PDF-1.7 canary document of Robin"))
	svg := putImage(t, e.alex, pathAvatar, file("image/svg+xml", []byte(`<svg><script>alert(1)</script></svg>`)))
	if svg.Status != http.StatusUnsupportedMediaType {
		t.Errorf("SVG: %d", svg.Status)
	}
	id := setAvatar(t, e.alex, mimeJPEG, hugeJPEG(t)).AvatarFileID
	if r := e.waitJobDone(t, id); r.UploadState != stateComplete || e.thumbnails(t, id) != 0 || !e.logLine(id, classTooManyPixels) {
		t.Errorf("huge JPEG: %+v", r)
	}
	if r := putImage(t, e.alex, pathAvatar, file(mimeJPEG, bytes.Repeat([]byte{0xFF}, partBytes+1))); r.Status != http.StatusRequestEntityTooLarge {
		t.Errorf("over the cap: %d", r.Status)
	}
}

// testAvatarAndLogoFileEvents: every file.updated of an avatar or logo file reaches only its
// uploader (audience), has an empty scope, kind file, receive action file.read and no JSON null;
// user.updated and instance.updated carry no null either.
func testAvatarAndLogoFileEvents(t *testing.T, e *avatarEnv) {
	rows, err := e.db.Query(t.Context(), `SELECT ev.audience_principal_id::text, f.uploaded_by::text, ev.space_id IS NULL AND ev.channel_id IS NULL
		AND ev.thread_id IS NULL, ev.resource_kind, ev.receive_action, ev.payload
		FROM event_log ev JOIN files f ON f.id = ev.resource_id
		WHERE ev.type = 'file.updated' AND f.purpose IN ('avatar', 'branding')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var audience *string
		var uploader, kind, receive string
		var empty bool
		var payload []byte
		if err := rows.Scan(&audience, &uploader, &empty, &kind, &receive, &payload); err != nil {
			t.Fatal(err)
		}
		n++
		if audience == nil || *audience != uploader || !empty || kind != "file" || receive != "file.read" {
			t.Errorf("file.updated: audience %v uploader %s empty scope %v kind %s receive %s", audience, uploader, empty, kind, receive)
		}
		noNull(t, "file.updated", payload)
		var p struct {
			File map[string]any `json:"file"`
		}
		if err := json.Unmarshal(payload, &p); err != nil || p.File["purpose"] == nil || p.File["original_name"] == nil {
			t.Errorf("FilePayload %s", payload)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no file.updated of an avatar or logo file")
	}
	for _, ev := range e.eventsOf(t, "user.updated", e.alex.PrincipalID) {
		noNull(t, "user.updated", ev.Payload)
	}
	for _, ev := range e.eventsOf(t, "instance.updated", e.instanceID(t)) {
		noNull(t, "instance.updated", ev.Payload)
	}
}

// TestAvatarLogoScanner runs the cases of a scanner verdict against the scripted clamd of M3-04:
// file.denied_mime_types with image/png, a scanner that is not ready (the file stays pending, the
// logo 404) and an infected logo (never delivered, 404).
func TestAvatarLogoScanner(t *testing.T) {
	fake, err := clamdtest.NewFake()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fake.Close() })
	cfg := config()
	cfg["file.scan.mode"] = "required"
	cfg["file.scan.clamd_address"] = fake.Address()
	cfg["file.scan.health_interval"] = "1s"
	cfg["file.scan.retry_backoff_cap"] = "2s"
	cfg["file.denied_mime_types"] = mimePNG
	s := harness.Start(t, harness.Options{Config: cfg})
	w := harness.BuildMatrixWorld(t, s)
	e := &avatarEnv{env: &env{s: s, alex: w.People["Alex"], sam: w.People["Sam"], db: s.AdminDB(), logs: &safeBuffer{}}, owner: s.Owner}
	e.noLogo = e.getLogo(t)
	jpg := encodeJPEG(t, twoColour(8, 8, colourFirst, colourSecond))

	r := putImage(t, e.alex, pathAvatar, file(mimePNG, encodePNG(t, twoColour(4, 4, colourFirst, colourSecond))))
	if r.Status != http.StatusUnsupportedMediaType || problemOf(t, r).Reason != "file_type_denied" {
		t.Errorf("denied PNG: %d %s", r.Status, r.Body)
	}

	fake.SetScript(clamdtest.Script{PingReply: "NOT READY"})
	pending := e.uploadLogo(t, mimeJPEG, jpg).LogoFileID
	poll(t, "a retry of the scan of "+pending, func() bool {
		return e.rowCount(t, `SELECT count(*) FROM jobs.river_job WHERE kind = 'files.scan' AND args->>'file_id' = $1`, pending) > 1
	})
	if row := e.readFile(t, pending); row.UploadState != statePending || row.ScanStatus != statePending {
		t.Fatalf("not ready: %+v", row)
	}
	sameAnswer(t, "pending logo and no logo", e.getLogo(t), e.noLogo)

	fake.SetScript(clamdtest.Script{Reply: "stream: Local.Test.Canary FOUND"})
	infected := e.uploadLogo(t, mimeJPEG, jpg).LogoFileID
	poll(t, "the scan of "+infected, func() bool { return e.readFile(t, infected).ScanStatus == "infected" })
	sameAnswer(t, "infected logo and no logo", e.getLogo(t), e.noLogo)
	if status, f := getFile(t, e.sam, infected); status != http.StatusOK || f.ScanStatus != "infected" {
		t.Errorf("getFile of the infected logo: %d %+v", status, f)
	}
	if slices.Contains(e.objects(t, infected), "medium") {
		t.Error("an infected logo has a variant")
	}
}
