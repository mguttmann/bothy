// SPDX-License-Identifier: Apache-2.0

package restclient

import (
	"errors"
	"net/http"
	"testing"
)

// TestDoerFunc: Do calls the function with the request and returns its result.
func TestDoerFunc(t *testing.T) {
	want := errors.New("sent")
	var got *http.Request
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://chat.example.com/api/v1/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	var d HttpRequestDoer = DoerFunc(func(r *http.Request) (*http.Response, error) { got = r; return nil, want })
	resp, err := d.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, want) || got != req {
		t.Errorf("Do = %v, request %v", err, got)
	}
}
