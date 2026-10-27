// SPDX-License-Identifier: Apache-2.0

// Package e2epush points the push adapters of the E2E server build at the fake providers of the
// mobile E2E stack (brief M5-11 Design note 1, infra/compose/test/compose.e2e-mobile.yaml): it
// reads the base URLs of Fake-APNs and Fake-FCM from two environment variables, both required, and
// sets the test-only endpoints of wire.Deps. Only the file of cmd/bothy with the Go build tag e2e imports
// it, so the release binary neither links this package nor reads its variables (a test holds
// that); no configuration key or flag reaches the endpoints. The fakes' certificate is trusted
// through SSL_CERT_FILE, which Go reads on Linux, so no root pool is set here.
//
// Example (cmd/bothy, build tag e2e):
//
//	deps := wire.Deps{Env: env}
//	if err := e2epush.Apply(env, &deps); err != nil {
//		return err
//	}
package e2epush

import (
	"errors"
	"net/url"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The environment variables of the E2E server build. They do not start with BOTHY_, so the
// configuration loader, which refuses unknown BOTHY_ variables, never sees them.
const (
	// EnvAPNs is the base URL of Fake-APNs for both APNs environments, for example
	// https://fakepush:8443.
	EnvAPNs = "E2E_PUSH_APNS_URL"
	// EnvFCM is the base URL of Fake-FCM, for example https://fakepush:8444; the token endpoint
	// is that URL plus /token, where Fake-FCM serves it and which the token_uri of its service
	// account names.
	EnvFCM = "E2E_PUSH_FCM_URL"
)

// apnsEnvironments are the values of push_registrations.environment of APNs (13 5.1); the fake
// serves both, so a debug build that registers sandbox reaches it as well.
var apnsEnvironments = []string{"production", "sandbox"}

// tokenPath is the path of the token endpoint below the base URL of Fake-FCM.
const tokenPath = "/token"

// Apply sets d.APNsEndpoints (production and sandbox) from EnvAPNs and d.FCMEndpoints (send and
// token) from EnvFCM. env is the environment of the process as KEY=VALUE entries, in which the
// last assignment wins. The build fails closed: both variables must be set, so a misconfigured
// run never leaves a provider on the real endpoints of Apple or Google, which it would reach with
// the fake's keys and the device tokens of the test.
//
// Errors: a variable that is missing, or whose value is not an https URL with a host and nothing
// after the port (no user info, path, query or fragment); the error names the variable, and d is
// unchanged.
func Apply(env []string, d *wire.Deps) error {
	apns, err := baseURL(env, EnvAPNs)
	if err != nil {
		return err
	}
	fcm, err := baseURL(env, EnvFCM)
	if err != nil {
		return err
	}
	d.APNsEndpoints = map[string]string{}
	for _, e := range apnsEnvironments {
		d.APNsEndpoints[e] = apns
	}
	d.FCMEndpoints = wire.FCMEndpoints{Send: fcm, Token: fcm + tokenPath}
	return nil
}

// baseURL returns the value of the variable name in env. The error reports a missing variable or
// a value that is not an https base URL; it names the variable, not the value.
func baseURL(env []string, name string) (string, error) {
	value, set := "", false
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, name+"="); ok {
			value, set = v, true
		}
	}
	if !set {
		return "", errors.New("e2epush: the E2E server build needs " + name + " (the base URL of the fake provider)")
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" ||
		u.Fragment != "" || u.ForceQuery || u.Opaque != "" || strings.HasSuffix(value, "#") {
		return "", errors.New("e2epush: " + name + " must be an https base URL with a host and nothing after the port")
	}
	return value, nil
}
