// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestReadyQueuedInReadOrder (M2-48): the ready of a login read before a confirmHost committed and
// the ready SendReady reads after the commit must reach the host in the order they were read, so
// the host keeps the newer declaration. Red before readyMu: the ready of the login was queued
// after the newer one and the host stayed on llm_credentials none.
func TestReadyQueuedInReadOrder(t *testing.T) {
	k := newLink(nil, 4, time.Second)
	entered, release := make(chan struct{}), make(chan struct{})
	loginDone := make(chan struct{})
	go func() {
		defer close(loginDone)
		_, _ = k.sendReady(func() (app.Ready, domain.CloseCode, error) {
			close(entered)
			<-release
			return app.Ready{State: "confirming", LLMCredentials: "none"}, 0, nil
		})
	}()
	<-entered
	resendDone := make(chan struct{})
	go func() {
		defer close(resendDone)
		_, _ = k.sendReady(func() (app.Ready, domain.CloseCode, error) {
			return app.Ready{State: "active", LLMCredentials: "organization"}, 0, nil
		})
	}()
	select {
	case <-resendDone: // without the order the newer ready is queued first
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	<-loginDone
	<-resendDone
	var last struct {
		D struct {
			LLM string `json:"llm_credentials"`
		} `json:"d"`
	}
	n := len(k.queue)
	for i := 0; i < n; i++ {
		if err := json.Unmarshal(<-k.queue, &last); err != nil {
			t.Fatal(err)
		}
	}
	if n != 2 || last.D.LLM != "organization" {
		t.Fatalf("%d readies, last llm_credentials %q, want 2 and organization", n, last.D.LLM)
	}
}
