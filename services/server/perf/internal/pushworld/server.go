// SPDX-License-Identifier: Apache-2.0

package pushworld

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"maps"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The test values of the APNs keys (brief M3-24 design note 6): no real Apple account or key.
const (
	teamID  = "EXAMPLETEAM"
	keyID   = "EXAMPLEKEY"
	keyFile = "apns/key.p8"
)

// StartServer generates a P-256 key per run, starts Fake-APNs with its public key and starts the
// harness server of opts in the test process (D-M3-24-1) with the APNs adapter enabled as in
// production (push.apns.team_id, key_id, private_key_ref and topics; the key as a file of the
// secrets directory, read by the file driver), pointed at the Fake (Options.APNsFake), and with
// every rate-limit key raised except ratelimit.rl.message.per_minute and .burst, which keep
// their defaults (D-M3-24-4). Entries of opts.Config win. Errors fail tb.
func StartServer(tb testing.TB, opts harness.Options) (*harness.Server, *apnstest.Server) {
	tb.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		tb.Fatal(err)
	}
	fake, err := apnstest.Start(apnstest.Options{TeamID: teamID, KeyID: keyID, PublicKey: &key.PublicKey})
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(fake.Close)
	cfg := RateLimits()
	cfg["push.apns.team_id"], cfg["push.apns.key_id"], cfg["push.apns.topics"] = teamID, keyID, appID
	cfg["push.apns.private_key_ref"] = "file:<secrets.dir>/" + keyFile
	maps.Copy(cfg, opts.Config)
	opts.Config = cfg
	files := map[string][]byte{keyFile: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})}
	maps.Copy(files, opts.SecretFiles)
	opts.SecretFiles = files
	opts.APNsFake = fake
	return harness.Start(tb, opts), fake
}

// RateLimits returns harness.RaisedRateLimits without the two keys of rl.message: the seeding
// sends no message, and the measured sends keep the default limit of their class (05 section 3).
func RateLimits() map[string]string {
	cfg := harness.RaisedRateLimits()
	delete(cfg, "ratelimit.rl.message.per_minute")
	delete(cfg, "ratelimit.rl.message.burst")
	return cfg
}

// DefaultOf returns the default of the configuration key name as text, from the server's schema.
func DefaultOf(tb testing.TB, name string) string {
	tb.Helper()
	schema, err := wire.ConfigSchema()
	if err != nil {
		tb.Fatal(err)
	}
	values, err := config.Load(schema, config.Sources{Env: []string{config.EnvName("server.public_url") + "=http://127.0.0.1:1"}})
	if err != nil {
		tb.Fatal(err)
	}
	return values.Text(name)
}
