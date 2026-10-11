// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-no-model-credentials (run by `make semgrep`).
package fixtures

import "os"

func config() {
	// ruleid: bothy-go-no-model-credentials
	_ = os.Getenv("ANTHROPIC_API_KEY")
	// ruleid: bothy-go-no-model-credentials
	_, _ = os.LookupEnv("CLAUDE_CODE_OAUTH_TOKEN")
	// ok: bothy-go-no-model-credentials
	_ = os.Getenv("BOTHY_DATABASE_URL")
}
