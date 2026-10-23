// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// terminalWait bounds each wait of the terminal step with Noise (a test constant).
const terminalWait = testkit.Patience

// TerminalIO is what the terminal step with Noise saw (M2-46 Design note 13): the positive controls
// of TestSEC_T10_TerminalInputOutput and the number of relay messages it added to the sinks.
type TerminalIO struct {
	// InputAtHost: the Fake-Host decrypted the canary input; OutputAtDevice: the device decrypted
	// the canary output; TokenInMessage1: the session token was the payload of message 1; Closed:
	// the frame close ended the session and the device stream closed with 4100.
	InputAtHost, OutputAtDevice, TokenInMessage1, Closed bool
	// RelayMessages is the number of binary messages added to SinkRelay.
	RelayMessages int
}

// terminalSessions drives the terminal sessions of M2-44 (INT-W49/3) and the terminal content of
// M2-46: the owner creates, pairs and confirms a host with a canary name and canary reasons, a
// Fake-Host with a real X25519 key connects it, and the owner opens a shell there, lists and reads
// the session, attaches to it and closes it with a canary reason; a second shell runs the Noise
// handshake and carries a canary input and a canary output (terminalIO). The session tokens of the
// grants are secrets: never in any sink.
func (sc *Scenario) terminalSessions() {
	sc.T.Helper()
	o := sc.W.Owner
	h := obj(sc.post(o, "/hosts", map[string]any{"name": sc.Reg.New(ClassContent, "", "terminal host name"), "reason": sc.Reg.New(ClassAdminReason, "", "terminal host create reason")}, created...))
	host := str(h, "host_id")
	if host == "" {
		host = str(h, "id")
	}
	code := str(obj(sc.post(o, "/hosts/"+host+"/pairing-code", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "terminal host pairing code reason")}, created...)), "pairing_code")
	sc.Reg.Register(code, ClassSecret, "", "terminal host pairing code")
	key := fakehost.NewNoiseKey()
	var jwk map[string]any
	if err := json.Unmarshal(key.Login.JWK(), &jwk); err != nil {
		sc.T.Fatal(err)
	}
	profiles := []any{map[string]any{"name": "shell", "display_name": "Shell", "kind": "shell"}}
	users := []any{map[string]any{"os_user": "bothy-s-owner", "workdirs": []string{}}}
	body, _ := json.Marshal(map[string]any{
		"pairing_code": code, "noise_public_key": fakehost.B64(key.Public[:]), "auth_public_jwk": jwk,
		"platform": "linux", "host_version": "1.0.0", "profiles": profiles, "session_users": users,
	})
	paired := sc.raw(http.MethodPost, "/hosts/pair", nil, body, "host service")
	if paired.Status != http.StatusCreated {
		sc.T.Errorf("pairHost: %d", paired.Status)
		return
	}
	prefix := strings.ReplaceAll(str(obj(paired), "key_fingerprint"), "-", "")
	if len(prefix) < 8 {
		sc.T.Errorf("pairHost: no key_fingerprint in %s", clip(paired.Body))
		return
	}
	path := "/hosts/" + host
	sc.matched(o, path, req{method: http.MethodPost, path: path + "/confirm", body: map[string]any{
		"fingerprint_prefix": strings.ToLower(prefix[:8]), "llm_credentials": "organization", "declared_server_host": false,
		"session_users": []any{map[string]any{"user_id": o.PrincipalID, "os_user": "bothy-s-owner"}},
		"reason":        sc.Reg.New(ClassAdminReason, "", "terminal host confirm reason"),
	}}, ok...)
	var doctor []any
	for _, c := range []string{"hd1_user", "hd2_groups", "hd3_sudo", "hd4_state_dir", "hd5_server_host", "hd6_llm_login",
		"hd7_profiles", "hd8_separation", "hd9_foreign_homes", "hd10_helper_integrity", "hd11_shared_locations"} {
		doctor = append(doctor, map[string]any{"check": c, "ok": true})
	}
	fh := fakehost.Connect(sc.T, sc.Server.PublicURL, ids.MustParse(host), key.Login, map[string]any{"profiles": profiles,
		"session_users": users, "doctor": doctor, "is_server_host": false, "require_local_confirm": false})
	fh.UseNoise(key)
	fh.Default(fakehost.Script{Kind: fakehost.NoiseAccept})
	cek := fakehost.B64(make([]byte, 32))
	grant := obj(sc.post(o, path+"/terminal/sessions", map[string]any{"profile": "shell", "cols": 80, "rows": 24, "client_ephemeral_key_hash": cek}, created...))
	sc.secretToken(grant)
	session := str(grant, "session.id")
	if session == "" {
		sc.T.Errorf("openTerminalSession: no session id in %v", grant)
		return
	}
	sc.get(o, "/terminal/sessions", ok...)
	sc.get(o, "/terminal/sessions/"+session, ok...)
	sc.secretToken(obj(sc.post(o, "/terminal/sessions/"+session+"/attach", map[string]any{"cols": 100, "rows": 30, "client_ephemeral_key_hash": cek}, ok...)))
	sc.post(o, "/terminal/sessions/"+session+"/close", map[string]any{"reason": sc.Reg.New(ClassAdminReason, "", "terminal session close reason")}, ok...)
	sc.terminalIO(o, ids.MustParse(host), fh)
}

// terminalIO opens a second shell of o on host with the hash of a fresh ephemeral key, runs the
// Noise handshake on the device stream (message 1 carries the session token), sends start, a data
// frame with a canary input and a resize, receives the Fake-Host's data frame with a canary output
// and closes with the frame close. The canaries are content of a scope no observer reads (the
// session), so even an event may not carry them. The raw binary messages of both sides go into
// SinkRelay; the positive controls into sc.TerminalIO.
func (sc *Scenario) terminalIO(o *harness.Client, host ids.ID, fh *fakehost.Host) {
	sc.T.Helper()
	eph := fakehost.NewEphemeral()
	// session.open reaches the Fake-Host before the answer of openTerminalSession, so the output is
	// its default script; the canary gets the scope of the session once its ID is known.
	output := sc.Reg.New(ClassContent, "", "terminal output")
	fh.Default(fakehost.Script{Kind: fakehost.NoiseAccept, Output: []byte("$ " + output + "\r\n")})
	grant := obj(sc.post(o, "/hosts/"+host.String()+"/terminal/sessions", map[string]any{"profile": "shell", "cols": 80, "rows": 24,
		"client_ephemeral_key_hash": eph.Hash()}, created...))
	sc.secretToken(grant)
	session, token := str(grant, "session.id"), str(grant, "session_token")
	static, err := base64.RawURLEncoding.DecodeString(str(grant, "noise_public_key"))
	if session == "" || token == "" || err != nil || len(static) != 32 {
		sc.T.Errorf("openTerminalSession for the Noise step: session %q, noise_public_key %d bytes, %v", session, len(static), err)
		return
	}
	id := ids.MustParse(session)
	input := sc.Reg.New(ClassContent, session, "terminal input")
	sc.Reg.Rescope(output, session)
	ctx, cancel := context.WithTimeout(sc.Ctx, terminalWait)
	defer cancel()
	dev, status, err := fakehost.DialDevice(sc.T, ctx, sc.Server.PublicURL, sc.Server.PublicURL, id, o,
		fakehost.DeviceParams{DeviceID: o.DeviceID, ClientIP: o.ClientIP})
	if err != nil {
		sc.T.Errorf("terminal device stream: %d %v", status, err)
		return
	}
	defer sc.relayRecords(fh, dev, id)
	p := fakehost.NoiseParams{HostID: host, Ephemeral: eph, Token: token}
	copy(p.HostStatic[:], static)
	stream, err := dev.Handshake(ctx, p)
	if err != nil {
		sc.T.Errorf("terminal noise handshake: %v", err)
		return
	}
	for _, f := range [][]byte{fakehost.StartFrame(80, 24), fakehost.DataFrame([]byte(input + "\r")), fakehost.ResizeFrame(100, 30)} {
		if err := stream.Send(ctx, f); err != nil {
			sc.T.Errorf("terminal frame: %v", err)
			return
		}
	}
	typ, got, err := stream.RecvFrame(ctx)
	sc.TerminalIO.OutputAtDevice = err == nil && typ == fakehost.FrameData && strings.Contains(string(got), output)
	if err := stream.Send(ctx, fakehost.CloseFrame()); err != nil {
		sc.T.Errorf("terminal frame close: %v", err)
		return
	}
	code, reason := dev.CloseCode(ctx)
	sc.TerminalIO.Closed = code == 4100 && reason == "client_closed"
	for _, f := range fh.Plain(id) {
		if f[0] == fakehost.FrameData && strings.Contains(string(f[1:]), input) {
			sc.TerminalIO.InputAtHost = true
		}
	}
	for _, t := range fh.Tokens() {
		sc.TerminalIO.TokenInMessage1 = sc.TerminalIO.TokenInMessage1 || t == token
	}
}

// relayRecords adds the raw binary messages of the device stream and of the data connection of
// session to SinkRelay.
func (sc *Scenario) relayRecords(fh *fakehost.Host, dev *fakehost.Device, session ids.ID) {
	add := func(who string, msgs []fakehost.WireMessage) {
		for i, m := range msgs {
			dir := "received"
			if m.Sent {
				dir = "sent"
			}
			sc.Sinks.Add(Record{Sink: SinkRelay, Source: fmt.Sprintf("terminal %s message %d %s", who, i, dir),
				Fields: []Field{{Location: "/bytes", Text: string(m.Bytes)}}})
			sc.TerminalIO.RelayMessages++
		}
	}
	add("device", dev.Wire())
	if dc := fh.Data(session); dc != nil {
		add("host", dc.Wire())
	}
}

// secretToken registers the session token of a terminal grant as a secret of the run.
func (sc *Scenario) secretToken(grant map[string]any) {
	sc.T.Helper()
	if tok := str(grant, "session_token"); tok != "" {
		sc.Reg.Register(tok, ClassSecret, "", "terminal session token")
	} else {
		sc.T.Errorf("terminal grant without session_token: %v", grant)
	}
}
