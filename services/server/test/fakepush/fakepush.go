// SPDX-License-Identifier: Apache-2.0

// Package fakepush is the fake push provider of the mobile device E2E test (brief M5-11 Design
// note 1; docs/spec/13-push-notifications.md 5 and 6): Fake-APNs of M3-20 (apnstest) and
// Fake-FCM of M3-21 (fcmtest) in one program that runs as the Compose service fakepush of
// infra/compose/test/compose.e2e-mobile.yaml. At start it generates the material of the run into
// a directory (the APNs key, the FCM service account and a per-run CA with a TLS leaf for the
// host name of the fakes), verifies what the real providers verify (the ES256 provider JWT of
// APNs, the RS256 assertion and the bearer token of FCM) with the generated keys, answers every
// valid send with 200, and records every accepted send for the capture API: plain HTTP with
// GET /healthz and GET /captured?after=<seq>, which the push relay of the runner reads to hand the
// payloads to the simulator and the emulator.
//
// It never reaches the real providers and holds no real credential. In the container it listens
// on the Compose network only; the overlay publishes the capture port on 127.0.0.1 of the host
// and nothing else. The server binary never links this package (it lives under test/, which no
// production package imports).
//
// Example:
//
//	s, err := fakepush.Start(fakepush.Options{Dir: "/push", Host: "fakepush",
//		APNsAddr: ":8443", FCMAddr: ":8444", CaptureAddr: ":8090"})
//	if err != nil {
//		return err
//	}
//	defer s.Close()
package fakepush

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm/fcmtest"
)

// The example identity of the run, which the overlay configures for the server (push.apns.*,
// push.fcm.*); no real Apple team, key or Firebase project has these values.
const (
	// APNsTeamID is push.apns.team_id, the claim iss Fake-APNs expects.
	APNsTeamID = "EXAMPLETEAM"
	// APNsKeyID is push.apns.key_id, the header kid Fake-APNs expects.
	APNsKeyID = "EXAMPLEKEY"
	// FCMProjectID is push.fcm.project_id, the project of the send path of Fake-FCM.
	FCMProjectID = "example-chat"
	// FCMClientEmail is client_email of the service account, the claim iss Fake-FCM expects.
	FCMClientEmail = "push@example-chat.example.com"
	// FCMKeyID is private_key_id of the service account, the header kid Fake-FCM expects.
	FCMKeyID = "EXAMPLEKEY"
)

// readHeaderTimeout bounds the request header of the capture API, so an idle client cannot hold
// a connection open forever.
const readHeaderTimeout = 10 * time.Second

// shutdownGrace is how long Close waits for running requests of the capture API.
const shutdownGrace = 5 * time.Second

// Options configure a run.
type Options struct {
	// Dir receives the files of the run (FileAPNsKey, FileFCMAccount, FileCA); it must exist. In
	// the image it is /push, the mount point of the named volume the server reads.
	Dir string
	// Host is the host name the server reaches both fakes under, named in their certificate and
	// URLs, for example "fakepush".
	Host string
	// APNsAddr and FCMAddr are the TLS listen addresses of Fake-APNs (HTTP/2) and Fake-FCM
	// (HTTP/1.1), for example ":8443" and ":8444"; CaptureAddr is the plain HTTP listen address
	// of the capture API, for example ":8090".
	APNsAddr, FCMAddr, CaptureAddr string
}

// Server is a running fakepush. It is safe for concurrent use.
type Server struct {
	apns    *apnstest.Server
	fcm     *fcmtest.Server
	capture *http.Server
	ln      net.Listener
	served  chan struct{}

	mu    sync.Mutex
	items []json.RawMessage
}

// Start generates the material of a run into opts.Dir, starts Fake-APNs and Fake-FCM under the
// run's leaf for opts.Host, writes the key, the service account and the CA, and then starts the
// capture API, so /healthz answers only once both fakes listen and every file is complete.
//
// Errors: a missing directory, a failed key generation, a listen address that cannot be bound
// and a file that cannot be written; what was started is closed again.
func Start(opts Options) (*Server, error) {
	if fi, err := os.Stat(opts.Dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("fakepush: the directory %q does not exist", opts.Dir)
	}
	m, err := generate(opts.Host)
	if err != nil {
		return nil, err
	}
	s := &Server{served: make(chan struct{})}
	s.apns, err = apnstest.Start(apnstest.Options{TeamID: APNsTeamID, KeyID: APNsKeyID, PublicKey: &m.apnsKey.PublicKey,
		Addr: opts.APNsAddr, Host: opts.Host, Certificate: &m.leaf, OnAccept: s.acceptAPNs})
	if err != nil {
		return nil, err
	}
	s.fcm, err = fcmtest.Start(fcmtest.Options{ProjectID: FCMProjectID, ClientEmail: FCMClientEmail, KeyID: FCMKeyID,
		PublicKey: &m.fcmKey.PublicKey, Addr: opts.FCMAddr, Host: opts.Host, Certificate: &m.leaf, OnAccept: s.acceptFCM})
	if err != nil {
		s.apns.Close()
		return nil, err
	}
	if err := s.writeFiles(opts.Dir, m); err != nil {
		s.closeFakes()
		return nil, err
	}
	var lc net.ListenConfig
	s.ln, err = lc.Listen(context.Background(), "tcp", opts.CaptureAddr)
	if err != nil {
		s.closeFakes()
		return nil, fmt.Errorf("fakepush: listen on %s: %w", opts.CaptureAddr, err)
	}
	s.capture = &http.Server{Handler: s.routes(), ReadHeaderTimeout: readHeaderTimeout}
	go func() {
		defer close(s.served)
		_ = s.capture.Serve(s.ln)
	}()
	return s, nil
}

// writeFiles writes the APNs key and the service account (mode 0600) and the CA (mode 0644)
// into dir.
func (s *Server) writeFiles(dir string, m *material) error {
	key, err := m.apnsKeyPEM()
	if err != nil {
		return err
	}
	account := s.fcm.ServiceAccountJSON(m.fcmKey)
	if account == nil {
		return errors.New("fakepush: the service account cannot be encoded")
	}
	return errors.Join(writeFile(dir, FileAPNsKey, key, 0o600), writeFile(dir, FileFCMAccount, account, 0o600),
		writeFile(dir, FileCA, m.caPEM, 0o644))
}

// APNsURL returns the base URL of Fake-APNs, https://<Host>:<port>.
func (s *Server) APNsURL() string { return s.apns.URL() }

// FCMURL returns the base URL of Fake-FCM, https://<Host>:<port>; its token endpoint is
// FCMURL() + "/token".
func (s *Server) FCMURL() string { return s.fcm.URL() }

// CaptureURL returns the base URL of the capture API, http://<listen address>.
func (s *Server) CaptureURL() string { return "http://" + s.ln.Addr().String() }

// Close stops the capture API, waiting up to a few seconds for running requests, and both
// fakes. The error reports a capture API that did not stop in time.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	err := s.capture.Shutdown(ctx)
	<-s.served
	s.closeFakes()
	return err
}

// closeFakes stops both fakes.
func (s *Server) closeFakes() {
	s.apns.Close()
	s.fcm.Close()
}

// apnsItem is a captured send to Fake-APNs in the form of the capture API.
type apnsItem struct {
	Seq      int64           `json:"seq"`
	Provider string          `json:"provider"`
	Token    string          `json:"token"`
	Topic    string          `json:"topic"`
	PushType string          `json:"push_type"`
	Priority string          `json:"priority"`
	Payload  json.RawMessage `json:"payload"`
}

// fcmItem is a captured send to Fake-FCM in the form of the capture API.
type fcmItem struct {
	Seq      int64             `json:"seq"`
	Provider string            `json:"provider"`
	Token    string            `json:"token"`
	Data     map[string]string `json:"data"`
	Android  json.RawMessage   `json:"android,omitempty"`
}

// acceptAPNs captures a send that Fake-APNs accepted: device token, topic, push type, priority
// and the body as sent. A body that is not JSON (the adapter never sends one) is kept as a JSON
// string, so the answer of the capture API stays valid JSON.
func (s *Server) acceptAPNs(r apnstest.Request) {
	payload := json.RawMessage(r.Body)
	if !json.Valid(r.Body) {
		payload, _ = json.Marshal(string(r.Body))
	}
	s.add(func(seq int64) any {
		return apnsItem{Seq: seq, Provider: "apns", Token: r.Token, Topic: r.Header.Get("Apns-Topic"),
			PushType: r.Header.Get("Apns-Push-Type"), Priority: r.Header.Get("Apns-Priority"), Payload: payload}
	})
}

// acceptFCM captures a send that Fake-FCM accepted: message.token, message.data and
// message.android when present. Data that is not an object of strings (the adapter never sends
// it) becomes an empty object.
func (s *Server) acceptFCM(r fcmtest.Request) {
	var body struct {
		Message struct {
			Token   string          `json:"token"`
			Data    json.RawMessage `json:"data"`
			Android json.RawMessage `json:"android"`
		} `json:"message"`
	}
	_ = json.Unmarshal(r.Body, &body)
	data := map[string]string{}
	if json.Unmarshal(body.Message.Data, &data) != nil || data == nil {
		data = map[string]string{}
	}
	android := body.Message.Android
	if string(android) == "null" {
		android = nil
	}
	s.add(func(seq int64) any {
		return fcmItem{Seq: seq, Provider: "fcm", Token: body.Message.Token, Data: data, Android: android}
	})
}

// add numbers an item with the next seq, starting at 1, and keeps its JSON. The lock orders the
// sends of both fakes into one sequence.
func (s *Server) add(item func(seq int64) any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.Marshal(item(int64(len(s.items)) + 1))
	if err != nil {
		// Unreachable: the items hold strings, maps of strings and valid JSON only. A placeholder
		// keeps the numbering, so a reader never waits for a seq that never comes.
		b = []byte(`{"seq":` + strconv.Itoa(len(s.items)+1) + `,"provider":"invalid"}`)
	}
	s.items = append(s.items, b)
}

// routes returns the handler of the capture API.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /captured", s.handleCaptured)
	return mux
}

// handleCaptured answers GET /captured?after=<seq> with {"items":[...]}: the captured sends with
// a seq greater than after (default 0), in increasing seq. An after that is not one whole number
// of at least 0 is 400.
func (s *Server) handleCaptured(w http.ResponseWriter, r *http.Request) {
	after := int64(0)
	if vs, ok := r.URL.Query()["after"]; ok {
		n, err := strconv.ParseInt(vs[0], 10, 64)
		if len(vs) != 1 || err != nil || n < 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"after must be one whole number of at least 0"}`))
			return
		}
		after = n
	}
	s.mu.Lock()
	var page []json.RawMessage
	if after < int64(len(s.items)) {
		page = append(page, s.items[after:]...)
	}
	s.mu.Unlock()
	if page == nil {
		page = []json.RawMessage{}
	}
	b, err := json.Marshal(struct {
		Items []json.RawMessage `json:"items"`
	}{page})
	if err != nil {
		http.Error(w, "cannot encode the items", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}
