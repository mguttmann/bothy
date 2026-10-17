// SPDX-License-Identifier: Apache-2.0

package pairing

import "github.com/OWNER/bothy/services/host/internal/noise"

func noiseFromPrivate(p [32]byte) ([]byte, error) {
	kp, err := noise.KeyPairFromPrivate(p)
	return kp.Public[:], err
}
