// SPDX-License-Identifier: Apache-2.0

package bothyapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// defaultDeviceName is the device name without a host name (docs/DECISIONS.md D-M8-10-6).
const defaultDeviceName = "bothy import"

// LoginOptions are the settings of Login.
type LoginOptions struct {
	// ServerURL is the origin of the server, like server.public_url: https, or http for a
	// loopback host (requirement 15); no path, query, fragment or user info.
	ServerURL string
	// DeviceName is the name of the device; "" means "bothy import on <Hostname>".
	DeviceName string
	// Hostname is the host name for the default device name; "" means "bothy import".
	Hostname string
	// SessionFile is the path of the session file.
	SessionFile string
	// AppVersion is the build version of the tool (app_version of the registration).
	AppVersion string
	// Limits are the tool values; see DefaultLimits.
	Limits Limits
	// Deps are the dependencies.
	Deps Deps
	// Stdout receives the pairing code alone; Stderr the instruction and the result. nil
	// discards.
	Stdout, Stderr io.Writer
}

// Login logs the tool in as a device with platform cli through device pairing (22 10.1, 08 5.3
// and 6.4): it refuses when a valid session for the same server exists, checks the server URL,
// generates an Ed25519 DPoP key and a UUIDv7 device ID, starts the pairing (beginPairing, no
// step-up key), prints the pairing code alone on Stdout and the instruction on Stderr, and polls
// completePairing every LoginPollInterval until the owner approves the code in the app or the
// web console. Only then it writes the session file. It never asks for or stores a password,
// passkey or step-up.
//
// Errors: ErrInvalidServerURL, ErrAlreadyLoggedIn, ErrInsecureSessionFile, ErrPairingExpired
// (expires_at passed), ErrPairingGone (410), ErrInterrupted (ctx cancelled; nothing written),
// the errors of the transport, or a *StatusError.
func Login(ctx context.Context, opts LoginOptions) error {
	if err := opts.Limits.validate(); err != nil {
		return err
	}
	origin, err := checkServerURL(opts.ServerURL)
	if err != nil {
		return err
	}
	stdout, stderr := writerOr(opts.Stdout), writerOr(opts.Stderr)
	file := SessionFile{Path: opts.SessionFile}
	if err := preflightSession(file, origin, opts.Deps); err != nil {
		return err
	}
	key, err := newProofKey()
	if err != nil {
		return err
	}
	co, err := newCore(origin, file, key, nil, opts.Limits, opts.Deps)
	if err != nil {
		return err
	}
	deviceID := ids.New()
	reg, err := registration(key, deviceID, opts)
	if err != nil {
		return err
	}
	begin, err := co.api.BeginPairingWithResponse(ctx, &restclient.BeginPairingParams{}, restclient.BeginPairingJSONRequestBody{Device: reg})
	if err != nil {
		return interrupted(ctx, err)
	}
	if begin.JSON201 == nil {
		return statusError("beginPairing", begin.HTTPResponse, begin.Body)
	}
	started := begin.JSON201
	_, _ = fmt.Fprintln(stdout, started.PairingCode)
	_, _ = fmt.Fprintf(stderr, "In the app or the web console choose Add device and enter this code; it expires at %s.\n",
		started.ExpiresAt.UTC().Format(time.RFC3339))
	return poll(ctx, co, started, origin, deviceID, opts.Limits.LoginPollInterval, stderr)
}

// poll calls completePairing every interval until the approval (the session is stored), a 410,
// the end of the pairing window or the end of ctx.
func poll(ctx context.Context, co *core, started *restclient.PairingStarted, origin string, deviceID ids.ID, interval time.Duration, stderr io.Writer) error {
	expires := started.ExpiresAt
	for {
		if err := co.sleep(ctx, interval); err != nil {
			return interrupted(ctx, err)
		}
		if !co.clk.Now().Before(expires) {
			return ErrPairingExpired
		}
		resp, err := co.api.CompletePairingWithResponse(ctx, &restclient.CompletePairingParams{},
			restclient.CompletePairingJSONRequestBody{PairingId: started.PairingId})
		if err != nil {
			return interrupted(ctx, err)
		}
		if ctx.Err() != nil {
			return ErrInterrupted
		}
		switch {
		case resp.JSON202 != nil:
			expires = resp.JSON202.ExpiresAt
		case resp.JSON201 != nil:
			s := sessionFrom(&Session{Version: sessionVersion, ServerURL: origin, DeviceID: deviceID.String(),
				UserID: resp.JSON201.Session.UserId.String(), DPoPPrivateJWK: co.key.privateJWK()}, resp.JSON201, co.clk.Now())
			if err := co.file.Store(s); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(stderr, "logged in")
			return nil
		case resp.StatusCode() == http.StatusGone:
			return ErrPairingGone
		default:
			return statusError("completePairing", resp.HTTPResponse, resp.Body)
		}
	}
}

// registration returns the device registration of a cli device: the DPoP key, no step-up key.
func registration(key *proofKey, deviceID ids.ID, opts LoginOptions) (restclient.DeviceRegistration, error) {
	var pub restclient.JsonObject
	if err := json.Unmarshal(key.publicJWK(), &pub); err != nil {
		return restclient.DeviceRegistration{}, err
	}
	reg := restclient.DeviceRegistration{
		DeviceId: restclient.Uuid(deviceID), Name: deviceName(opts.DeviceName, opts.Hostname),
		Platform: restclient.DevicePlatform("cli"), PublicJwk: pub, Alg: restclient.DeviceRegistrationAlg("EdDSA"),
	}
	if opts.AppVersion != "" {
		v := opts.AppVersion
		reg.AppVersion = &v
	}
	return reg, nil
}

// preflightSession refuses a login while a valid session for the same server exists, and a
// session directory that group or others can access, before anything is sent.
func preflightSession(file SessionFile, origin string, deps Deps) error {
	s, err := file.Load()
	switch {
	case err == nil:
		now := time.Now()
		if deps.Clock != nil {
			now = deps.Clock.Now()
		}
		if s.ServerURL == origin && now.Before(s.RefreshExpiresAt) {
			return ErrAlreadyLoggedIn
		}
	case errors.Is(err, ErrInsecureSessionFile):
		return err
	}
	if _, err := os.Stat(filepath.Dir(file.Path)); err == nil {
		return file.checkDir()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Logout ends the session (22 10.1): under the lock of the session file it calls logout, then
// deletes the session file and releases the lock; the lock file stays. A session the server
// already revoked still deletes the file. Errors: ErrNotLoggedIn without a session file, the
// errors of New, of the transport (the file stays), or a *StatusError.
func Logout(ctx context.Context, opts Options) error {
	c, err := New(ctx, opts)
	if err != nil {
		return err
	}
	unlock, err := c.core.file.Lock()
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	c.core.lockHeld.Store(true)
	// Another process may have rotated the tokens or logged out since New read the file.
	disk, err := c.core.file.Load()
	if errors.Is(err, ErrNotLoggedIn) {
		return ErrNotLoggedIn
	}
	if err != nil {
		return err
	}
	c.core.setSession(disk)
	// The DPoP logout of a device sends no X-Console-Request (INT-W34/2).
	resp, err := c.LogoutWithResponse(ctx, &restclient.LogoutParams{})
	switch {
	case errors.Is(err, ErrSessionRevoked):
	case err != nil:
		return err
	case resp.StatusCode() == http.StatusNoContent, resp.StatusCode() == http.StatusUnauthorized:
	default:
		return statusError("logout", resp.HTTPResponse, resp.Body)
	}
	return c.core.file.Remove()
}

// checkServerURL returns the origin of raw (scheme and host in lower case, no trailing slash):
// an https URL, or http with a loopback host (127.0.0.0/8, ::1, localhost); no path except "/",
// no query, fragment or user info. Errors: ErrInvalidServerURL.
func checkServerURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Opaque != "" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", ErrInvalidServerURL
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "https":
	case "http":
		if !loopbackHost(u.Hostname()) {
			return "", ErrInvalidServerURL
		}
	default:
		return "", ErrInvalidServerURL
	}
	return scheme + "://" + strings.ToLower(u.Host), nil
}

// loopbackHost reports whether host is localhost or a loopback address.
func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	a, err := netip.ParseAddr(host)
	return err == nil && a.Unmap().IsLoopback()
}

// deviceName returns name, else "bothy import on <hostname>", else "bothy import"; a name with
// control or bidi characters falls back to the plain default.
func deviceName(name, hostname string) string {
	if name = strings.TrimSpace(name); name == "" {
		name = defaultDeviceName
		if h := strings.TrimSpace(hostname); h != "" {
			name += " on " + h
		}
	}
	if textcheck.DisplayText(name) != nil {
		return defaultDeviceName
	}
	return name
}

// interrupted maps an error after a cancelled ctx to ErrInterrupted.
func interrupted(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ErrInterrupted
	}
	return err
}

// writerOr returns w, or io.Discard for nil.
func writerOr(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
