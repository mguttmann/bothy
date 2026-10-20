// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

// images drives avatar and logo of M3-10 (INT-W51/13): Alex sets and deletes the avatar, the
// owner uploads the instance logo (fresh step-up), reads it through getBrandingLogo (200 once the
// logo is deliverable, else the same 404 as without a logo) and deletes it again.
func (sc *Scenario) images() {
	sc.T.Helper()
	alex, o := sc.W.People["Alex"], sc.W.Owner
	sc.call(alex, sc.imagePart(http.MethodPut, "/me/avatar"), ok...)
	sc.del(alex, "/me/avatar", none...)
	sc.call(o, sc.imagePart(http.MethodPut, "/instance/branding/logo"), ok...)
	sc.get(o, "/instance/branding/logo", http.StatusOK, http.StatusNotFound)
	sc.del(o, "/instance/branding/logo", none...)
}

// imagePart is a request with one multipart part "file": a small PNG.
func (sc *Scenario) imagePart(method, path string) req {
	sc.T.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for x := range 8 {
		for y := range 8 {
			img.Set(x, y, color.NRGBA{R: uint8(30 * x), G: uint8(30 * y), B: 90, A: 255})
		}
	}
	var pic bytes.Buffer
	if err := png.Encode(&pic, img); err != nil {
		sc.T.Fatal(err)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="image.png"`)
	h.Set("Content-Type", "image/png")
	pw, err := w.CreatePart(h)
	if err == nil {
		_, err = pw.Write(pic.Bytes())
	}
	if err == nil {
		err = w.Close()
	}
	if err != nil {
		sc.T.Fatal(err)
	}
	return req{method: method, path: path, body: buf.Bytes(), header: header("Content-Type", w.FormDataContentType())}
}
