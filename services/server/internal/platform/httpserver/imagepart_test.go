// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// imagePart is one part of a test body.
type imagePart struct{ name, ctype, data string }

// multipartReader builds a multipart reader over parts.
func multipartReader(t testing.TB, parts ...imagePart) *multipart.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="`+p.name+`"; filename="me.png"`)
		if p.ctype != "" {
			h.Set("Content-Type", p.ctype)
		}
		pw, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = pw.Write([]byte(p.data))
	}
	_ = w.Close()
	return multipart.NewReader(&buf, w.Boundary())
}

// fieldOf returns the one field error of err.
func fieldOf(t *testing.T, err error) errs.FieldError {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 {
		t.Fatalf("got %v, want one field error of validation_failed", err)
	}
	return e.Fields[0]
}

// TestImagePart holds the framing of ImageUpload (brief M3-10 design note 2 a and d): the first
// part must be "file"; its reader ends with io.EOF only without a further part.
func TestImagePart(t *testing.T) {
	ctype, body, err := httpserver.ImagePart(multipartReader(t, imagePart{"file", "image/png", "pixels"}))
	if err != nil || ctype != "image/png" {
		t.Fatalf("%q %v", ctype, err)
	}
	b, err := io.ReadAll(body)
	if err != nil || string(b) != "pixels" {
		t.Fatalf("%q %v", b, err)
	}

	_, body, err = httpserver.ImagePart(multipartReader(t, imagePart{"file", "image/png", "pixels"}, imagePart{"extra", "text/plain", "x"}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(body)
	if f := fieldOf(t, err); f.Pointer != "/extra" || f.Code != "unknown_member" {
		t.Fatalf("further part: %+v", f)
	}

	for name, mr := range map[string]*multipart.Reader{
		"no part":         multipartReader(t),
		"other name":      multipartReader(t, imagePart{"image", "image/png", "pixels"}),
		"nil reader":      nil,
		"file not first":  multipartReader(t, imagePart{"other", "", "x"}, imagePart{"file", "image/png", "pixels"}),
		"broken framing":  multipart.NewReader(strings.NewReader("no boundary here"), "b"),
		"empty body":      multipart.NewReader(strings.NewReader(""), "b"),
		"header overflow": multipart.NewReader(strings.NewReader("--b\r\n"+strings.Repeat("X-A: b\r\n", 3)), "b"),
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := httpserver.ImagePart(mr)
			if f := fieldOf(t, err); f.Pointer != "/file" || (f.Code != "required" && f.Code != "invalid") {
				t.Fatalf("%+v", f)
			}
		})
	}
}

// TestSetFileHeaders holds the one writer of SEC-HDR-4: disposition, both filename parameters,
// the sandbox policy and nosniff; a name outside ASCII keeps only its encoded form.
func TestSetFileHeaders(t *testing.T) {
	h := http.Header{}
	httpserver.SetFileHeaders(h, "image/png", "logo.png", true)
	for k, v := range map[string]string{
		"Content-Type": "image/png", "Content-Disposition": `inline; filename="logo.png"; filename*=UTF-8''logo.png`,
		"Content-Security-Policy": "default-src 'none'; sandbox", "X-Content-Type-Options": "nosniff",
	} {
		if h.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, h.Get(k), v)
		}
	}
	h = http.Header{}
	httpserver.SetFileHeaders(h, "application/pdf", "Bericht \"Sam\" ü.pdf", false)
	want := `attachment; filename="Bericht _Sam_ __.pdf"; filename*=UTF-8''Bericht%20%22Sam%22%20%C3%BC.pdf`
	if got := h.Get("Content-Disposition"); got != want {
		t.Errorf("Content-Disposition = %q, want %q", got, want)
	}
}

// FuzzImageUploadParts feeds random multipart bodies through ImagePart and reads the part to its
// end like the avatar and logo use cases: every refusal is a 4xx error of kernel/errs, never a
// server error or a panic (brief M3-10, test FuzzImageUploadParts).
func FuzzImageUploadParts(f *testing.F) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.SetBoundary("fuzzboundary")
	pw, _ := w.CreateFormFile("file", "a.png")
	_, _ = pw.Write([]byte("\x89PNG"))
	_ = w.Close()
	f.Add(buf.Bytes())
	f.Add([]byte("--fuzzboundary\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\nx\r\n--fuzzboundary\r\n" +
		"Content-Disposition: form-data; name=\"y\"\r\n\r\nz\r\n--fuzzboundary--\r\n"))
	f.Add([]byte("--fuzzboundary\r\n\r\n"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, body []byte) {
		_, part, err := httpserver.ImagePart(multipart.NewReader(bytes.NewReader(body), "fuzzboundary"))
		if err == nil {
			_, err = io.ReadAll(io.LimitReader(part, 1<<20))
		}
		if err == nil {
			return
		}
		e, ok := errs.As(err)
		if !ok || e.Kind().Status() >= http.StatusInternalServerError {
			t.Fatalf("server error %v for %q", err, body)
		}
	})
}
