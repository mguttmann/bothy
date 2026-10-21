// SPDX-License-Identifier: Apache-2.0

package fcm

import (
	"strings"
	"testing"
)

func TestConfigEnabledAndValidate(t *testing.T) {
	if !(Config{ProjectID: "example-chat"}).Enabled() {
		t.Fatal("a project ID does not enable FCM")
	}
	if (Config{}).Enabled() {
		t.Fatal("an empty project ID enables FCM")
	}
	for _, ok := range []string{"", "example-chat", "a", "chat-2"} {
		if err := (Config{ProjectID: ok}).Validate(); err != nil {
			t.Fatalf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"Example-Chat", "1chat", "chat/x", "chat project", "-chat", "chat_x"} {
		err := ValidateProjectID(bad)
		if err == nil {
			t.Fatalf("%q passes", bad)
		}
		if !strings.Contains(err.Error(), KeyProjectID) || strings.Contains(err.Error(), bad) {
			t.Fatalf("%q: message %q must name the key and not the value", bad, err)
		}
	}
}
