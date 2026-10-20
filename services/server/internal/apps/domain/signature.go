// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The signature schemes of the header webhook-signature (Standard Webhooks, ADR 0016): v1a is
// Ed25519 with the server key of purpose webhook_out, v1 is HMAC-SHA256 with the derived secret.
const (
	SchemeV1a = "v1a"
	SchemeV1  = "v1"
)

// SecretPrefix is the prefix of the v1 secret shown once to the owner (whsec_ and standard base64,
// the form the Standard Webhooks libraries decode).
const SecretPrefix = "whsec_"

// SignedContent returns the bytes both schemes sign: "{webhook-id}.{webhook-timestamp}.{body}" with
// the event ID in canonical form, the Unix seconds of the attempt and the body bytes as sent
// (webhooks.md section 2).
func SignedContent(webhookID ids.ID, ts int64, body []byte) []byte {
	head := webhookID.String() + "." + strconv.FormatInt(ts, 10) + "."
	return append([]byte(head), body...)
}

// hmacLabel starts the derivation input, separating this use of the master key from any other.
const hmacLabel = "webhook-v1:"

// DeriveHMACKey returns the v1 key of a subscription generation (docs/DECISIONS.md D-M2-26-10):
// HMAC-SHA256(master, "webhook-v1:" + subscription id in canonical lower-case form + ":" +
// generation in decimal). The key is never stored; the same inputs always give the same key.
func DeriveHMACKey(master []byte, subscriptionID ids.ID, generation int) []byte {
	m := hmac.New(sha256.New, master)
	m.Write([]byte(hmacLabel + subscriptionID.String() + ":" + strconv.Itoa(generation)))
	return m.Sum(nil)
}

// FormatSecret returns key in the form shown once to the owner: whsec_ and standard base64 with
// padding.
func FormatSecret(key []byte) string { return SecretPrefix + base64.StdEncoding.EncodeToString(key) }

// ErrSecretFormat reports a value that is not whsec_ and standard base64.
var ErrSecretFormat = errors.New("apps: the value is no whsec_ secret")

// ParseSecret returns the key bytes of a whsec_ secret. Errors: ErrSecretFormat; it never repeats
// the value.
func ParseSecret(s string) ([]byte, error) {
	rest, ok := strings.CutPrefix(s, SecretPrefix)
	if !ok {
		return nil, ErrSecretFormat
	}
	key, err := base64.StdEncoding.DecodeString(rest)
	if err != nil || len(key) == 0 {
		return nil, ErrSecretFormat
	}
	return key, nil
}

// HMACSignature returns the v1 signature of content with key (HMAC-SHA256).
func HMACSignature(key, content []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(content)
	return m.Sum(nil)
}

// SignatureHeader returns the value of webhook-signature: entries "<scheme>,<standard base64>"
// separated by one space, every v1a entry first in signer order, then the v1 entries in the order
// given (newest generation first; docs/DECISIONS.md D-M2-26-10).
func SignatureHeader(v1a [][]byte, v1 [][]byte) string {
	parts := make([]string, 0, len(v1a)+len(v1))
	for _, s := range v1a {
		parts = append(parts, SchemeV1a+","+base64.StdEncoding.EncodeToString(s))
	}
	for _, s := range v1 {
		parts = append(parts, SchemeV1+","+base64.StdEncoding.EncodeToString(s))
	}
	return strings.Join(parts, " ")
}

// HMACGenerations returns the generations the server signs v1 with at now: the current one, and
// the previous one while rotatedAt + webhook.secret_rotation_overlap lies ahead; never older ones
// (webhooks.md section 2.1 point 3). Newest first.
func HMACGenerations(generation int, rotatedAt time.Time, overlap time.Duration, now time.Time) []int {
	if generation > 0 && !rotatedAt.IsZero() && now.Before(rotatedAt.Add(overlap)) {
		return []int{generation, generation - 1}
	}
	return []int{generation}
}
