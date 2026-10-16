// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/bothyapi/bothytest"
)

// TestNoSecretInOutput (requirement 14, 22 10.2 rule 4): with a fake that echoes the presented
// tokens and proofs into every error body, no access token, refresh token, private key or session
// file content appears in the debug log, standard output, standard error, an error text or a
// print form of a result.
func TestNoSecretInOutput(t *testing.T) {
	e := newEnv(t)
	e.fake.Echo(true)
	e.login(t)
	var secrets []string
	collect := func() {
		raw, err := os.ReadFile(e.path)
		if err != nil {
			return
		}
		secrets = append(secrets, string(raw))
		var s struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			Key          struct {
				D string `json:"d"`
			} `json:"dpop_private_jwk"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		secrets = append(secrets, s.AccessToken, s.RefreshToken, s.Key.D)
	}
	collect()
	c, err := New(t.Context(), e.options())
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	record := func(v any, err error) {
		fmt.Fprintf(&out, "%v %+v %#v\n", v, v, v)
		if err != nil {
			fmt.Fprintf(&out, "%v %+v %#v %s\n", err, err, err, err.Error())
		}
	}
	fmt.Fprintf(&out, "%v %+v %#v\n", c, c, c)
	record(c.Me(t.Context()))
	e.fake.RotateNonce()
	record(c.Me(t.Context()))
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		e.fake.Fail(bothytest.PathMe, status, "", "", 1)
		record(c.Me(t.Context()))
	}
	e.fake.ExpireAccess()
	record(c.Me(t.Context()))
	collect()
	e.clk.Advance(bothytest.AccessTTL + time.Second)
	e.fake.Fail(bothytest.PathRefresh, http.StatusUnauthorized, "", "", 1)
	record(c.Me(t.Context()))
	record(c.Limits(t.Context()))
	record(nil, Logout(t.Context(), e.options()))
	record(nil, Logout(t.Context(), e.options()))
	all := out.String() + e.log.String() + e.out.String() + e.errw.String()
	if !strings.Contains(e.log.String(), "level=DEBUG") {
		t.Fatal("no debug records were written")
	}
	for _, s := range secrets {
		if s != "" && strings.Contains(all, s) {
			t.Errorf("a secret appears in the output: %.12s...", s)
		}
	}
	if strings.Contains(all, "DPoP at-") || strings.Contains(all, "rt-") {
		t.Error("a token form appears in the output")
	}
}
