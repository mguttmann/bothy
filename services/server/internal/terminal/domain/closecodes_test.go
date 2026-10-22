// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestStreamCodeNamesMatchAsyncAPI holds the names of the device stream codes, the reason text of
// a close frame without a detail, equal to x-close-codes of the channel terminalStream.
func TestStreamCodeNamesMatchAsyncAPI(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "api", "asyncapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, "\n  terminalStream:\n")
	if i < 0 {
		t.Fatal("no channel terminalStream")
	}
	section := s[i+1:]
	if j := regexp.MustCompile(`\n  [A-Za-z]+:\n`).FindStringIndex(section[1:]); j != nil {
		section = section[:j[0]+1]
	}
	contract := map[domain.StreamCode]string{}
	for _, m := range regexp.MustCompile(`- code: (\d+)\n\s+name: ([a-z_]+)`).FindAllStringSubmatch(section, -1) {
		code, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatal(err)
		}
		contract[domain.StreamCode(code)] = m[2]
	}
	for _, c := range []domain.StreamCode{
		domain.StreamNormal, domain.StreamDecodeError, domain.StreamAuthFailed, domain.StreamRevoked, domain.StreamServerRestart,
		domain.StreamSlowConsumer, domain.StreamTerminalClosed, domain.StreamTokenInvalid, domain.StreamHostUnavailable,
		domain.StreamReplaced, domain.StreamHandshakeFailed,
	} {
		want, ok := contract[c]
		if !ok {
			t.Errorf("code %d is not in x-close-codes of terminalStream", c)
			continue
		}
		if got := c.Name(); got != want {
			t.Errorf("code %d: Name() = %q, asyncapi %q", c, got, want)
		}
	}
}
