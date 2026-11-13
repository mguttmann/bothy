// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestTopicLimitEqualsServerDefault: the tool cuts a channel topic to importtool.TopicMaxChars,
// because getInstance does not carry channel.topic_max_chars. This holds the constant equal to the
// default of a started server, so a changed default fails here instead of in an import.
func TestTopicLimitEqualsServerDefault(t *testing.T) {
	t.Parallel()
	s := harness.Start(t, harness.Options{EgressDenyAll: true})
	if got := s.ConfigValue("channel.topic_max_chars"); got != strconv.Itoa(importtool.TopicMaxChars) {
		t.Errorf("the default of channel.topic_max_chars is %s, the tool cuts a topic to %d characters", got, importtool.TopicMaxChars)
	}
}
