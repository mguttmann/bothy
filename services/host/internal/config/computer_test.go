// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"testing"
)

// TestComputerKeys: the three keys of docs/spec/27-computer-use.md 18 with their defaults.
func TestComputerKeys(t *testing.T) {
	c, err := Parse([]byte("host_service: {}\n"), linux)
	if err != nil {
		t.Fatal(err)
	}
	if c.ComputerUse || c.ComputerFrameMaxBytes != DefaultComputerFrameMaxBytes || c.ComputerTestPeer != "" {
		t.Errorf("defaults %v %d %q", c.ComputerUse, c.ComputerFrameMaxBytes, c.ComputerTestPeer)
	}
	if DefaultComputerFrameMaxBytes != 67108864 {
		t.Errorf("default frame max %d", DefaultComputerFrameMaxBytes)
	}
	c, err = Parse([]byte("host_service:\n  computer_use: true\n  computer_frame_max_bytes: 1048576\n  computer_test_peer: alex\n"), linux)
	if err != nil {
		t.Fatal(err)
	}
	if !c.ComputerUse || c.ComputerFrameMaxBytes != 1048576 || c.ComputerTestPeer != "alex" {
		t.Errorf("set %v %d %q", c.ComputerUse, c.ComputerFrameMaxBytes, c.ComputerTestPeer)
	}
}

// TestComputerKeyRules: a frame limit below one channel frame, computer_test_peer outside Linux,
// and computer_use together with owner_account are refused, naming the key (27 9.4, 18).
func TestComputerKeyRules(t *testing.T) {
	cases := []struct {
		name, body string
		p          Platform
		key        string
	}{
		{"frame max zero", "host_service:\n  computer_frame_max_bytes: 0\n", linux, "host_service.computer_frame_max_bytes"},
		{"test peer on macOS", "host_service:\n  computer_test_peer: alex\n", Platform{GOOS: "darwin"}, "host_service.computer_test_peer"},
		{"with owner account", "host_service:\n  owner_account: alex\n  computer_use: true\n", Platform{GOOS: "darwin"}, "host_service.computer_use"},
		{"owner account first", "host_service:\n  computer_use: true\n  owner_account: alex\n", linux, "host_service.computer_use"},
		{"not a bool", "host_service:\n  computer_use: yes please\n", linux, "host_service.computer_use"},
	}
	for _, c := range cases {
		_, err := Parse([]byte(c.body), c.p)
		var ke *KeyError
		if !errors.As(err, &ke) || ke.Key != c.key {
			t.Errorf("%s: %v, want key %s", c.name, err, c.key)
		}
	}
}

// TestComputerOwnerAccountMessage: the refusal names both keys (27 9.4).
func TestComputerOwnerAccountMessage(t *testing.T) {
	_, err := Parse([]byte("host_service:\n  owner_account: alex\n  computer_use: true\n"), linux)
	if err == nil || !contains(err.Error(), "host_service.owner_account") || !contains(err.Error(), "host_service.computer_use") {
		t.Errorf("error %v names not both keys", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
