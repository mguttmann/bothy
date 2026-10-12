// SPDX-License-Identifier: Apache-2.0

package egress_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// A consumer allows the hosts it needs with a policy; every other host is refused before any
// DNS lookup or connection.
func ExampleNew() {
	cfg := egress.Config{Timeout: 15 * time.Second, MaxResponseBytes: 1 << 20}
	client, err := egress.New(cfg, egress.Options{
		Purpose: "oidc",
		Hosts: func(_ context.Context, scheme, host string, port uint16) bool {
			return scheme == "https" && host == "idp.example.com" && port == 443
		},
		Redirects: egress.RedirectChecked,
	}, egress.Deps{})
	if err != nil {
		fmt.Println(err)
		return
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://other.example.com/.well-known/openid-configuration", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	var blocked *egress.BlockedError
	if errors.As(err, &blocked) {
		fmt.Println(blocked.Reason, blocked.Host)
	}
	// Output: host_not_allowed other.example.com
}

func ExampleParseRule() {
	r, err := egress.ParseRule("[2001:db8::10]:8443;allow_private")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(r.Host, r.Port, r.AllowPrivate)
	fmt.Println(r)
	// Output:
	// 2001:db8::10 8443 true
	// [2001:db8::10]:8443;allow_private
}
