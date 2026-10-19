// SPDX-License-Identifier: Apache-2.0

package s3mirror

// NewWithDestinationEndpoint is New with the destination endpoint replaced, so the tests can
// mirror into a plain-http SeaweedFS; production destinations are always https
// (domain.OffsiteTarget.Endpoint).
func NewWithDestinationEndpoint(cfg Config, endpoint string) (*Mirror, error) {
	return newMirror(cfg, endpoint)
}
