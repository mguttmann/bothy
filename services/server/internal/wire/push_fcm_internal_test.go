// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"io"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// TestFCMDepsCarryOnlyTestOverrides (brief M5-11 Design note 1): the FCM adapter of a process
// built the way cmd/bothy builds it gets no endpoint override, so it targets Google only; the
// override Deps.FCMEndpoints of the E2E server build reaches the adapter unchanged.
func TestFCMDepsCarryOnlyTestOverrides(t *testing.T) {
	t.Parallel()
	eg := egress.Config{AllowedHosts: []egress.Rule{{Host: "fakepush", Port: 8444, AllowPrivate: true}}}
	prod := (&server{role: RoleWorker, deps: Deps{Env: []string{}, Stdout: io.Discard, Version: "test"}}).fcmDeps(eg)
	if prod.Endpoints != (FCMEndpoints{}) {
		t.Fatalf("production FCM deps carry endpoints %+v", prod.Endpoints)
	}
	if len(prod.Egress.AllowedHosts) != 1 || prod.Egress.AllowedHosts[0] != eg.AllowedHosts[0] {
		t.Fatalf("production FCM deps lost the egress configuration: %+v", prod.Egress)
	}
	endpoints := FCMEndpoints{Send: "https://fakepush:8444", Token: "https://fakepush:8444/token"} // #nosec G101 -- URLs of the fake, no credential
	test := (&server{role: RoleWorker, deps: Deps{FCMEndpoints: endpoints}}).fcmDeps(eg)
	if test.Endpoints != endpoints {
		t.Fatalf("test FCM deps endpoints %+v, want %+v", test.Endpoints, endpoints)
	}
}
