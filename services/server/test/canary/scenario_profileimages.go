// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// profileImages drives the avatar and the instance logo (M3-10): Alex sets and removes an avatar,
// the owner uploads a logo (with step-up), anonymous callers read it with getBrandingLogo once the
// jobs made it deliverable (ScenarioConfig switches the scan off), and the owner removes it again,
// after which getBrandingLogo answers 404. The original file name carries a canary of the
// uploader's file scope: the server stores the fixed names avatar and logo, so it may reach no
// sink and no answer of another observer.
func (sc *Scenario) profileImages() {
	sc.T.Helper()
	alex, o := sc.W.People["Alex"], sc.W.Owner
	name := sc.Reg.New(ClassContent, fileScope, "image file name") + ".png"
	sc.call(alex, sc.imageReq("/me/avatar", name), ok...)
	sc.del(alex, "/me/avatar", none...)

	sc.call(o, sc.imageReq("/instance/branding/logo", name), ok...)
	logo := req{method: http.MethodGet, path: "/instance/branding/logo"}
	testkit.Until(sc.T, "the logo of the canary run becomes deliverable", func() bool {
		return sc.anon(alex, logo, http.StatusOK, http.StatusNotFound).Status == http.StatusOK
	})
	sc.del(o, "/instance/branding/logo", none...)
	sc.anon(alex, logo, http.StatusNotFound)
}

// imageReq is a PUT of path with a multipart body whose one part file is a small PNG named name.
func (sc *Scenario) imageReq(path, name string) req {
	sc.T.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	draw.Draw(img, image.Rect(0, 0, 4, 8), image.NewUniform(color.RGBA{R: 200, G: 40, B: 40, A: 255}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(4, 0, 8, 8), image.NewUniform(color.RGBA{R: 40, G: 40, B: 200, A: 255}), image.Point{}, draw.Src)
	var pic bytes.Buffer
	if err := png.Encode(&pic, img); err != nil {
		sc.T.Fatalf("encode the canary image: %v", err)
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+name+`"`)
	h.Set("Content-Type", "image/png")
	pw, err := w.CreatePart(h)
	if err == nil {
		_, err = pw.Write(pic.Bytes())
	}
	if err == nil {
		err = w.Close()
	}
	if err != nil {
		sc.T.Fatalf("build the canary image body: %v", err)
	}
	return req{method: http.MethodPut, path: path, body: body.Bytes(), header: header("Content-Type", w.FormDataContentType())}
}
