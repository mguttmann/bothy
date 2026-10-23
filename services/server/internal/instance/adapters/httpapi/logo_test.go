// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakeLogos is the port instance.Logos with fixed answers; it records the callers and, when open
// is true, opens the image and keeps what it read.
type fakeLogos struct {
	branding  instance.Branding
	variant   instance.LogoVariant
	err       error
	open      bool
	declared  string
	read      string
	readErr   error
	callers   []actor.Actor
	removeHit int
}

func (f *fakeLogos) UploadLogo(_ context.Context, a actor.Actor, open instance.LogoOpener) (instance.Branding, error) {
	f.callers = append(f.callers, a)
	if !f.open {
		return instance.Branding{}, f.err
	}
	img, err := open()
	if err != nil {
		return instance.Branding{}, err
	}
	f.declared = img.DeclaredType
	b, err := io.ReadAll(img.Body)
	f.read, f.readErr = string(b), err
	return f.branding, f.err
}

func (f *fakeLogos) RemoveLogo(_ context.Context, a actor.Actor) error {
	f.callers = append(f.callers, a)
	f.removeHit++
	return f.err
}

func (f *fakeLogos) OpenLogo(_ context.Context, a actor.Actor) (instance.LogoVariant, error) {
	f.callers = append(f.callers, a)
	return f.variant, f.err
}

// closeRecorder is a variant body that records Close.
type closeRecorder struct {
	io.Reader
	closed bool
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}

// imageBody returns a multipart body with the parts (name, content type, bytes).
func imageBody(t *testing.T, parts ...[3]string) *multipart.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="`+p[0]+`"; filename="photo.png"`)
		h.Set("Content-Type", p[1])
		pw, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pw.Write([]byte(p[2])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return multipart.NewReader(&buf, w.Boundary())
}

// failReader fails the test when the handler reads the body.
type failReader struct{ t *testing.T }

func (r failReader) Read([]byte) (int, error) {
	r.t.Error("the request body was read")
	return 0, io.ErrUnexpectedEOF
}

func TestLogos_UploadOpensThePartAndConverts(t *testing.T) {
	t.Parallel()
	owner := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	logo := ids.New()
	f := &fakeLogos{open: true, branding: instance.Branding{ProductName: "Team", AccentColor: "loch", LogoFileID: logo, Version: 4}}
	ctx := actor.WithActor(context.Background(), owner)
	r, err := httpapi.NewLogos(f).UploadBrandingLogo(ctx, rest.UploadBrandingLogoRequestObject{Body: imageBody(t, [3]string{"file", "image/png", "png bytes"})})
	if err != nil {
		t.Fatal(err)
	}
	b := r.(rest.UploadBrandingLogo200JSONResponse).Body
	if b.ProductName != "Team" || *b.AccentColor != "loch" || b.LogoFileId == nil || *b.LogoFileId != rest.Uuid(logo) || b.Version != 4 {
		t.Errorf("branding %+v", b)
	}
	if f.declared != "image/png" || f.read != "png bytes" || f.readErr != nil || f.callers[0].ID != owner.ID {
		t.Errorf("opened %q %q %v by %+v", f.declared, f.read, f.readErr, f.callers)
	}

	// Without a logo the member is omitted; a second part ends the read with validation_failed.
	f = &fakeLogos{open: true, branding: instance.Branding{ProductName: "Team", Version: 5}}
	r, err = httpapi.NewLogos(f).UploadBrandingLogo(ctx, rest.UploadBrandingLogoRequestObject{Body: imageBody(t, [3]string{"file", "image/png", "x"})})
	if err != nil || r.(rest.UploadBrandingLogo200JSONResponse).Body.LogoFileId != nil {
		t.Errorf("zero logo: %+v %v", r, err)
	}
	f = &fakeLogos{open: true}
	_, _ = httpapi.NewLogos(f).UploadBrandingLogo(ctx, rest.UploadBrandingLogoRequestObject{
		Body: imageBody(t, [3]string{"file", "image/png", "x"}, [3]string{"other", "text/plain", "y"}),
	})
	if e, ok := errs.As(f.readErr); !ok || e.PublicCode() != errs.CodeValidationFailed {
		t.Errorf("second part: %v", f.readErr)
	}
	f = &fakeLogos{open: true}
	if _, err := httpapi.NewLogos(f).UploadBrandingLogo(ctx, rest.UploadBrandingLogoRequestObject{
		Body: imageBody(t, [3]string{"photo", "image/png", "x"}),
	}); code(err) != errs.CodeValidationFailed {
		t.Errorf("part not named file: %v", err)
	}
}

func TestLogos_UploadDeniedReadsNoByte(t *testing.T) {
	t.Parallel()
	f := &fakeLogos{err: errs.New(errs.CodeForbidden, "").WithReason("owner_only")}
	_, err := httpapi.NewLogos(f).UploadBrandingLogo(context.Background(), rest.UploadBrandingLogoRequestObject{
		Body: multipart.NewReader(failReader{t}, "boundary"),
	})
	if code(err) != errs.CodeForbidden {
		t.Errorf("denied: %v", err)
	}
	if f.callers[0].Kind != actor.KindAnonymous {
		t.Errorf("caller outside the chain %+v, want anonymous", f.callers[0])
	}
}

func TestLogos_Delete(t *testing.T) {
	t.Parallel()
	f := &fakeLogos{}
	r, err := httpapi.NewLogos(f).DeleteBrandingLogo(context.Background(), rest.DeleteBrandingLogoRequestObject{})
	if _, ok := r.(rest.DeleteBrandingLogo204Response); err != nil || !ok || f.removeHit != 1 {
		t.Errorf("delete %+v %v", r, err)
	}
	f.err = errs.New(errs.CodeStepUpRequired, "")
	if _, err := httpapi.NewLogos(f).DeleteBrandingLogo(context.Background(), rest.DeleteBrandingLogoRequestObject{}); code(err) != errs.CodeStepUpRequired {
		t.Errorf("error: %v", err)
	}
}

// getLogo serves getBrandingLogo over f and returns the recorded response.
func getLogo(t *testing.T, f *fakeLogos) (*httptest.ResponseRecorder, error) {
	t.Helper()
	r, err := httpapi.NewLogos(f).GetBrandingLogo(context.Background(), rest.GetBrandingLogoRequestObject{})
	if err != nil {
		return nil, err
	}
	w := httptest.NewRecorder()
	if err := r.VisitGetBrandingLogoResponse(w); err != nil {
		t.Fatal(err)
	}
	return w, nil
}

func TestLogos_GetBrandingLogoHeaders(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ ct, ext string }{{"image/png", "png"}, {"image/jpeg", "jpg"}} {
		data := "variant bytes of " + tc.ct
		body := &closeRecorder{Reader: strings.NewReader(data)}
		f := &fakeLogos{variant: instance.LogoVariant{Body: body, ContentType: tc.ct, SizeBytes: int64(len(data))}}
		w, err := getLogo(t, f)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(data))
		want := map[string]string{
			"Content-Type":            tc.ct,
			"Content-Disposition":     `inline; filename="logo.` + tc.ext + `"; filename*=UTF-8''logo.` + tc.ext,
			"Content-Security-Policy": "default-src 'none'; sandbox",
			"X-Content-Type-Options":  "nosniff",
			"Etag":                    `"` + hex.EncodeToString(sum[:]) + `"`,
			"Repr-Digest":             "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":",
			"Content-Length":          strconv.Itoa(len(data)),
		}
		for k, v := range want {
			if got := w.Header().Get(k); got != v {
				t.Errorf("%s: %s = %q, want %q", tc.ct, k, got, v)
			}
		}
		// Cache-Control and the CORS headers stay with the chain (no-store, SEC-CORS-2).
		for _, k := range []string{"Cache-Control", "Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Accept-Ranges"} {
			if v := w.Header().Get(k); v != "" {
				t.Errorf("%s: handler set %s = %q", tc.ct, k, v)
			}
		}
		if w.Code != http.StatusOK || w.Body.String() != data || !body.closed {
			t.Errorf("%s: status %d body %q closed %v", tc.ct, w.Code, w.Body.String(), body.closed)
		}
		if f.callers[0].Kind != actor.KindAnonymous {
			t.Errorf("caller %+v, want anonymous", f.callers[0])
		}
	}
}

func TestLogos_GetBrandingLogoRefusesAWrongSize(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		data string
		size int64
		ct   string
	}{
		"shorter":      {"abc", 4, "image/png"},
		"longer":       {"abcde", 4, "image/png"},
		"no size":      {"", 0, "image/png"},
		"other type":   {"abcd", 4, "image/webp"},
		"read failure": {"", 4, "image/png"},
	} {
		var r io.Reader = strings.NewReader(tc.data)
		if name == "read failure" {
			r = io.MultiReader(strings.NewReader("ab"), errReader{})
		}
		body := &closeRecorder{Reader: r}
		_, err := getLogo(t, &fakeLogos{variant: instance.LogoVariant{Body: body, ContentType: tc.ct, SizeBytes: tc.size}})
		want := errs.CodeUnavailable
		if name == "other type" {
			want = errs.CodeInternalError
		}
		if code(err) != want || !body.closed {
			t.Errorf("%s: %v closed %v", name, err, body.closed)
		}
	}
	nf := errs.New(errs.CodeNotFound, "")
	if _, err := getLogo(t, &fakeLogos{err: nf}); !errors.Is(err, nf) {
		t.Errorf("not_found: %v", err)
	}
}

// code returns the public problem code of err.
func code(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.PublicCode()
	}
	return ""
}

// errReader fails every read.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("disk gone") }
