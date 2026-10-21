// SPDX-License-Identifier: Apache-2.0

package title_test

import (
	"fmt"

	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
)

func ExamplePlaceholders_IsPlaceholder() {
	p := title.NewPlaceholders(title.DefaultEntries())
	for _, t := range []string{"", "  neuer   chat 3", "New chat", "Test2", "AI", "Invoice export fails"} {
		fmt.Printf("%q %v\n", t, p.IsPlaceholder(t))
	}
	// Output:
	// "" true
	// "  neuer   chat 3" true
	// "New chat" true
	// "Test2" false
	// "AI" false
	// "Invoice export fails" false
}
