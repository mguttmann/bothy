// SPDX-License-Identifier: Apache-2.0

package errs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestWithHeaderCarriesHeaders(t *testing.T) {
	t.Parallel()
	e := New(CodeUseDPoPNonce, "").
		WithHeader("WWW-Authenticate", `DPoP error="use_dpop_nonce"`).
		WithHeader("dpop-nonce", "n1")
	if got := e.Headers.Get("Www-Authenticate"); got != `DPoP error="use_dpop_nonce"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	if got := e.Headers.Get("DPoP-Nonce"); got != "n1" {
		t.Fatalf("DPoP-Nonce = %q", got)
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "n1") {
		t.Fatalf("headers must not be marshalled: %s", data)
	}
}

func TestVersionConflictAndPreconditionRequired(t *testing.T) {
	t.Parallel()
	vc := VersionConflict(map[string]any{"version": 8})
	if vc.PublicCode() != CodeVersionConflict || vc.Kind().Status() != 412 || vc.Current == nil {
		t.Fatalf("VersionConflict = %+v", vc)
	}
	pr := PreconditionRequired()
	if pr.PublicCode() != CodePreconditionRequired || pr.Kind().Status() != 428 {
		t.Fatalf("PreconditionRequired = %+v", pr)
	}
}

// TestTitlesMatchOpenAPI holds Title against x-problem-codes of the schema Problem in
// docs/api/openapi.yaml, the one source of titles (06 R-API-23).
func TestTitlesMatchOpenAPI(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas struct {
				Problem struct {
					Codes []struct {
						Code   string `yaml:"code"`
						Status int    `yaml:"status"`
						Title  string `yaml:"title"`
					} `yaml:"x-problem-codes"`
				} `yaml:"Problem"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	rows := doc.Components.Schemas.Problem.Codes
	if len(rows) != len(AllCodes()) {
		t.Fatalf("openapi.yaml lists %d codes, errs %d", len(rows), len(AllCodes()))
	}
	for _, r := range rows {
		c := Code(r.Code)
		if got := c.Title(); got != r.Title {
			t.Errorf("%s: Title() = %q, openapi.yaml says %q", c, got, r.Title)
		}
		if got := c.Kind().Status(); got != r.Status {
			t.Errorf("%s: status %d, openapi.yaml says %d", c, got, r.Status)
		}
	}
	if got := Code("no_such_code").Title(); got != "" {
		t.Errorf("unknown code: Title() = %q, want empty", got)
	}
}
