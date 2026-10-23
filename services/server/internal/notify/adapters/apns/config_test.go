// SPDX-License-Identifier: Apache-2.0

package apns_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns"
)

func TestConfigEnabledAndValidate(t *testing.T) {
	t.Parallel()
	full := apns.Config{TeamID: teamID, KeyID: keyID, PrivateKeyRef: "file:/run/secrets/apns_key.p8", Topics: []string{topicPhone, topicMac}}
	if !full.Enabled() || full.Validate() != nil {
		t.Fatalf("full: enabled %v, validate %v", full.Enabled(), full.Validate())
	}
	if none := (apns.Config{}); none.Enabled() || none.Validate() != nil {
		t.Fatalf("none: enabled %v, validate %v", none.Enabled(), none.Validate())
	}
	partial := []apns.Config{
		{TeamID: teamID},
		{KeyID: keyID},
		{Topics: []string{topicPhone}},
		{TeamID: teamID, KeyID: keyID},
		{TeamID: teamID, Topics: []string{topicPhone}},
		{KeyID: keyID, Topics: []string{topicPhone}},
	}
	for i, c := range partial {
		err := c.Validate()
		if c.Enabled() || err == nil {
			t.Errorf("partial %d: enabled %v, err %v", i, c.Enabled(), err)
			continue
		}
		for _, k := range []string{"push.apns.team_id", "push.apns.key_id", "push.apns.topics"} {
			if !strings.Contains(err.Error(), k) {
				t.Errorf("partial %d: %q does not name %s", i, err, k)
			}
		}
		if strings.Contains(err.Error(), teamID) || strings.Contains(err.Error(), keyID) || strings.Contains(err.Error(), topicPhone) {
			t.Errorf("partial %d: %q names a value", i, err)
		}
	}
	for name, topics := range map[string][]string{"empty": {topicPhone, ""}, "repeated": {topicPhone, topicPhone}} {
		c := full
		c.Topics = topics
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "push.apns.topics") {
			t.Errorf("%s topic: %v", name, err)
		}
	}
	noRef := full
	noRef.PrivateKeyRef = ""
	if err := noRef.Validate(); err == nil || !strings.Contains(err.Error(), "push.apns.private_key_ref") {
		t.Errorf("enabled without key reference: %v", err)
	}
}
