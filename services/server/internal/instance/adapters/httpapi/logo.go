// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Logos implements uploadBrandingLogo, deleteBrandingLogo and getBrandingLogo of
// rest.StrictServerInterface over instance.Logos (brief M3-10). The use cases authorize; the
// handlers open the image part only through the opener the use case calls after authorize, and
// write the file response of getBrandingLogo.
type Logos struct {
	logos instance.Logos
}

// NewLogos returns the handlers over the port.
//
// Example:
//
//	h := httpapi.NewLogos(admin) // admin is *app.Admin
func NewLogos(l instance.Logos) *Logos { return &Logos{logos: l} }

// UploadBrandingLogo implements rest.StrictServerInterface (PUT /instance/branding/logo, owner
// with step-up). The multipart body is opened lazily: httpserver.ImagePart runs only when the use
// case calls the opener, after authorize allowed (design note 5).
func (h *Logos) UploadBrandingLogo(ctx context.Context, req rest.UploadBrandingLogoRequestObject) (rest.UploadBrandingLogoResponseObject, error) {
	open := func() (instance.LogoImage, error) {
		declared, body, err := httpserver.ImagePart(req.Body)
		if err != nil {
			return instance.LogoImage{}, err
		}
		return instance.LogoImage{DeclaredType: declared, Body: body}, nil
	}
	b, err := h.logos.UploadLogo(ctx, principal(ctx), open)
	if err != nil {
		return nil, err
	}
	return rest.UploadBrandingLogo200JSONResponse{Body: toRESTBranding(b)}, nil
}

// DeleteBrandingLogo implements rest.StrictServerInterface (DELETE /instance/branding/logo, owner
// with step-up); idempotent.
func (h *Logos) DeleteBrandingLogo(ctx context.Context, _ rest.DeleteBrandingLogoRequestObject) (rest.DeleteBrandingLogoResponseObject, error) {
	if err := h.logos.RemoveLogo(ctx, principal(ctx)); err != nil {
		return nil, err
	}
	return rest.DeleteBrandingLogo204Response{}, nil
}

// logoExtensions maps the type of a logo variant to the extension of its file name.
var logoExtensions = map[string]string{"image/png": "png", "image/jpeg": "jpg"}

// GetBrandingLogo implements rest.StrictServerInterface (GET /instance/branding/logo, public). It
// reads the variant once, bounded by its stored size, because ETag and Repr-Digest must precede
// the body and file_thumbnails stores no hash (design note 8). Errors: those of the use case;
// 503 unavailable when the bytes do not match the stored size; internal for a variant type
// outside logoExtensions.
func (h *Logos) GetBrandingLogo(ctx context.Context, _ rest.GetBrandingLogoRequestObject) (rest.GetBrandingLogoResponseObject, error) {
	v, err := h.logos.OpenLogo(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	ext, ok := logoExtensions[v.ContentType]
	if !ok {
		_ = v.Body.Close()
		return nil, errs.New(errs.CodeInternalError, "instance: the logo variant has a type outside image/png and image/jpeg")
	}
	data, err := readVariant(v)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	return logoResponse{
		body: data, contentType: v.ContentType, filename: "logo." + ext,
		etag: `"` + hex.EncodeToString(sum[:]) + `"`, digest: "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":",
	}, nil
}

// readVariant reads the whole variant and closes it. At most SizeBytes plus one byte are read, so
// a body that differs from the stored size is detected without reading further. Errors: 503
// unavailable for a read failure or a size mismatch (the stored variant is not what its row says).
func readVariant(v instance.LogoVariant) ([]byte, error) {
	defer func() { _ = v.Body.Close() }()
	if v.SizeBytes < 1 {
		return nil, errs.New(errs.CodeUnavailable, "")
	}
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(v.Body, v.SizeBytes+1))
	if err != nil {
		return nil, errs.Wrap(errs.CodeUnavailable, err, "")
	}
	if n != v.SizeBytes {
		return nil, errs.New(errs.CodeUnavailable, "")
	}
	return buf.Bytes(), nil
}

// logoResponse is the 200 answer of getBrandingLogo with the headers of design note 8. It
// replaces the generated rest.GetBrandingLogo200ImageResponse, which writes neither the file
// headers nor ETag and Repr-Digest. Cache-Control (no-store) and Access-Control-Allow-Origin
// (SEC-CORS-2) come from the chain; Range is ignored.
type logoResponse struct {
	body        []byte
	contentType string
	filename    string
	etag        string
	digest      string
}

// VisitGetBrandingLogoResponse implements rest.GetBrandingLogoResponseObject.
func (r logoResponse) VisitGetBrandingLogoResponse(w http.ResponseWriter) error {
	h := w.Header()
	httpserver.SetFileHeaders(h, r.contentType, r.filename, true)
	h.Set("ETag", r.etag)
	h.Set("Repr-Digest", r.digest)
	h.Set("Content-Length", strconv.Itoa(len(r.body)))
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r.body)
	return err
}
