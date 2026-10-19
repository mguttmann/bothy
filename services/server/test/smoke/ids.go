// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"crypto/rand"
	"encoding/binary"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// newUUIDv7 returns a fresh UUIDv7 (RFC 9562 5.7): 48 bits of Unix milliseconds, version 7,
// random bits and the variant 10. device_id, client_msg_id and Idempotency-Key are UUIDv7 in the
// contract. It panics when the system has no randomness, like crypto/rand itself.
func newUUIDv7() rest.Uuid {
	var u [16]byte
	if _, err := rand.Read(u[6:]); err != nil {
		panic(err)
	}
	var ms [8]byte
	binary.BigEndian.PutUint64(ms[:], uint64(time.Now().UnixMilli())) // #nosec G115 -- the clock is after 1970
	copy(u[:6], ms[2:])
	u[6] = u[6]&0x0f | 0x70
	u[8] = u[8]&0x3f | 0x80
	return rest.Uuid(u)
}
