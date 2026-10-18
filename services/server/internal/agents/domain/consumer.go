// SPDX-License-Identifier: Apache-2.0

package domain

import "regexp"

// DefaultConsumer is the consumer name of an agent that names none at identify (04 section 6.3).
const DefaultConsumer = "default"

// consumerNamePattern is the pattern of the path parameter ConsumerPath and of
// GatewayIdentify.consumer in docs/api/openapi.yaml. Go's regexp runs in linear time (SEC-IN-6).
var consumerNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// ValidConsumerName reports whether name is a consumer name: a lower-case identifier that starts
// with a letter or digit, for example "default" or "worker-1".
func ValidConsumerName(name string) bool { return consumerNamePattern.MatchString(name) }

// CheckAck checks the acknowledgement seq of a consumer whose acknowledgement is acked against
// the head head of the serving process (brief M2-07 note 8; 04 section 15.2 item 1). It reports
// whether the acknowledgement advances: seq at or below acked changes nothing and is no error (06
// section 2 "No effect"), seq above acked advances, also beyond what was dispatched, because an
// acknowledgement covers every number up to seq. Errors: validation_failed on /seq for seq below 0
// or above head.
//
// Example:
//
//	advance, err := domain.CheckAck(seq, held, head)
func CheckAck(seq, acked, head int64) (bool, error) {
	if seq < 0 || seq > head {
		return false, fieldError("/seq", FieldInvalid, "the sequence number is below 0 or above the head")
	}
	return seq > acked, nil
}
