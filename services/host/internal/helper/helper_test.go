// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock/clocktest"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/hostos/hostostest"
	"github.com/OWNER/bothy/services/host/internal/localchan"
)

func world(t *testing.T) doctor.HelperEnv {
	t.Helper()
	w := hostostest.New("linux")
	w.AddUser("bothy-s-alex", "1001", "/home/bothy-s-alex", true)
	w.AddUser("bothy-s-sam", "1002", "/home/bothy-s-sam", true)
	w.Dir("/home/bothy-s-alex", "1001", 0o700)
	w.Dir("/home/bothy-s-sam", "1002", 0o700)
	w.File("/home/bothy-s-alex/x", "1002", 0o600, "")
	a, _ := w.Users().Lookup("bothy-s-alex")
	v := w.As("bothy-s-alex")
	return doctor.HelperEnv{GOOS: "linux", Self: a, Users: w.Users(), Security: w.Security(), Resolver: v, Files: v, Probe: v.Probe(),
		Runner: &hostostest.Runner{}, Clock: clocktest.New(time.Unix(0, 0))}
}

func call(t *testing.T, h localchan.Handler, op string, d any) (string, json.RawMessage) {
	t.Helper()
	b, _ := json.Marshal(d)
	gotOp, body := h(context.Background(), localchan.Message{Op: op, ID: "1", D: b})
	out, _ := json.Marshal(body)
	return gotOp, out
}

func TestHandler(t *testing.T) {
	env := world(t)
	h := Handler(env)
	if op, body := call(t, h, localchan.OpHello, nil); op != localchan.OpHello || string(body) != `{"session_user":"bothy-s-alex","version":1}` {
		t.Errorf("hello %s %s", op, body)
	}
	req := doctor.Request{Mode: doctor.ModeFull, SessionUser: "bothy-s-alex", Others: []string{"bothy-s-sam"}, ForeignHomes: []doctor.NamedHome{{Name: "bothy-s-sam", Home: "/home/bothy-s-sam"}},
		EntryScanTimeout: time.Minute}
	op, body := call(t, h, localchan.OpCheckRun, req)
	var res doctor.HelperResult
	if op != localchan.OpCheckResult || json.Unmarshal(body, &res) != nil || len(res.Listable) != 0 || res.PlaceCode != "" || !res.LLM.Unchecked {
		t.Errorf("check %s %s", op, body)
	}
	if op, body := call(t, h, localchan.OpScanRun, req); op != localchan.OpScanResult || string(body) != `{"code":"foreign_entry"}` {
		t.Errorf("scan %s %s", op, body)
	}
	req.SessionUser = "bothy-s-sam"
	if op, _ := call(t, h, localchan.OpCheckRun, req); op != "" {
		t.Error("a request for another session user answered")
	}
	if op, _ := call(t, h, "session.open", nil); op != "" {
		t.Error("unknown op answered")
	}
	if op, _ := h(context.Background(), localchan.Message{Op: localchan.OpCheckRun, D: json.RawMessage(`[`)}); op != "" {
		t.Error("malformed body answered")
	}
}
