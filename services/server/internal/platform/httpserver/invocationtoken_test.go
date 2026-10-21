// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// invocationAuthenticator records the Invocation-Token values it sees and answers with the
// anonymous actor, or with 401 unauthenticated when fail is set, so the chain's error path runs too.
type invocationAuthenticator struct {
	mu   sync.Mutex
	seen [][]string
	fail bool
}

// Authenticate records creds.InvocationTokens.
func (a *invocationAuthenticator) Authenticate(_ context.Context, creds actor.Credentials) (actor.Actor, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.seen = append(a.seen, slices.Clone(creds.InvocationTokens))
	if a.fail {
		return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "")
	}
	return actor.Anonymous(), nil
}

// Revalidate implements actor.Authenticator; the chain does not call it.
func (*invocationAuthenticator) Revalidate(context.Context, actor.Actor) error { return nil }

// TestAuth_InvocationTokenHeader (M2-14, 07 guard rule SR9): StageAuthenticate hands every value of
// the Invocation-Token header to the Authenticator in order, and no value reaches a log record or a
// span attribute, on success and on a refused request.
func TestAuth_InvocationTokenHeader(t *testing.T) {
	t.Parallel()
	httpservertest.Spans()
	op := getThing()
	auth := &invocationAuthenticator{}
	h := httpservertest.New(t, httpservertest.Options{Authenticator: auth, Routes: []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}}})
	const first, second = "eyJhbGciOiJFZERTQSJ9.eyJmaXJzdCI6MX0.Zmlyc3Qtc2ln", "eyJhbGciOiJFZERTQSJ9.eyJzZWNvbmQiOjJ9.c2Vjb25kLXNpZw"
	request := func() *http.Request {
		req := h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast", "Authorization", "Bearer agent-token")
		req.Header.Add("Invocation-Token", first)
		req.Header.Add("Invocation-Token", second)
		return req
	}
	h.Do(request())
	auth.mu.Lock()
	auth.fail = true
	auth.mu.Unlock()
	if resp := h.Do(request()); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("refused request: status %d, want 401", resp.StatusCode)
	}
	auth.mu.Lock()
	seen := slices.Clone(auth.seen)
	auth.mu.Unlock()
	if len(seen) != 2 || !slices.Equal(seen[0], []string{first, second}) || !slices.Equal(seen[1], []string{first, second}) {
		t.Fatalf("the Authenticator saw %d requests, the first with %d header values (want both values in order)", len(seen), len(firstOr(seen)))
	}
	secrets := []string{first, second}
	for _, v := range []string{first, second} {
		secrets = append(secrets, strings.Split(v, ".")[1:]...)
	}
	for _, s := range secrets {
		if h.Logs.Contains(s) {
			t.Fatal("an Invocation-Token value or segment appears in a log record")
		}
		for _, span := range httpservertest.Spans().Ended() {
			for _, kv := range span.Attributes() {
				if strings.Contains(kv.Value.String(), s) {
					t.Fatalf("an Invocation-Token value or segment appears in the span attribute %s", kv.Key)
				}
			}
		}
	}
}

// firstOr returns the first element of seen, or nil.
func firstOr(seen [][]string) []string {
	if len(seen) == 0 {
		return nil
	}
	return seen[0]
}
