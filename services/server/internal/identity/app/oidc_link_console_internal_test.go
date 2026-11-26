// SPDX-License-Identifier: Apache-2.0

package app

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// TestLinkOptsConsole: a link begun by a console session must name the console's link callback
// on the origin of server.public_url and stores it as the flow's redirect; an app session keeps
// the check against the provider's app_redirect_uris (no options; V1-14).
func TestLinkOptsConsole(t *testing.T) {
	t.Parallel()
	o := &OIDC{consoleLink: "https://bothy.example.com" + consoleLinkCallback}
	console := actor.Actor{Kind: actor.KindUser, ClientComponent: string(identity.ComponentConsole)}
	opt, err := o.linkOpts(console, "https://bothy.example.com/console/sign-in/callback")
	if err != nil || opt.redirect != o.consoleLink {
		t.Fatalf("linkOpts(console) = %+v, %v", opt, err)
	}
	for _, redirect := range []string{
		"",
		"https://bothy.example.com/console/login/callback",
		"https://other.example.com/console/sign-in/callback",
		"http://127.0.0.1:4711/callback",
	} {
		_, err := o.linkOpts(console, redirect)
		wantCode(t, err, errs.CodeValidationFailed)
	}
	app := actor.Actor{Kind: actor.KindUser, ClientComponent: string(identity.ComponentApp)}
	opt, err = o.linkOpts(app, "http://127.0.0.1:4711/callback")
	if err != nil || opt != (beginOpts{}) {
		t.Fatalf("linkOpts(app) = %+v, %v", opt, err)
	}
}
