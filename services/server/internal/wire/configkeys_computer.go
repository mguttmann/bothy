// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// computerConfigKeys returns the server keys of computer use (CU-04, docs/spec/27-computer-use.md
// section 18): the validity of an attach token, the deadline of the pairing, and the values the
// server sends to the host service with computer.open. Their defaults are only in chapter 27.
// Without a desktop whose helper the owner enabled at the machine, none of them changes anything
// (27 4, rule 11 of the brief).
func computerConfigKeys() []config.Key {
	return []config.Key{
		{Name: "computer.attach_token_ttl", Kind: config.KindDuration, Default: thirtySeconds, Min: "1s", Chapter: chapter27,
			Doc: "Validity of the single-use attach token of a computer session."},
		{Name: "computer.open_timeout", Kind: config.KindDuration, Default: twentySeconds, Min: "1s", Chapter: chapter27,
			Doc: "Deadline until both sides of a computer session are paired and the handshake is done."},
		{Name: "computer.image_max_edge", Kind: config.KindInt, Default: "1280", Min: "1", Chapter: chapter27,
			Doc: "Longest edge of a screenshot in pixels, sent to the host with computer.open."},
		{Name: "computer.jpeg_quality", Kind: config.KindInt, Default: "80", Min: "1", Max: "100", Chapter: chapter27,
			Doc: "JPEG quality of a screenshot."},
		{Name: "computer.settle", Kind: config.KindDuration, Default: "500ms", Min: "0s", Chapter: chapter27,
			Doc: "Wait after an action before its automatic image."},
		{Name: "computer.auto_screenshot", Kind: config.KindBool, Default: defaultTrue, Chapter: chapter27,
			Doc: "An image after every action that changes the screen."},
		{Name: "computer.wait_max", Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: chapter27,
			Doc: "Longest wait of one action of a computer session."},
		{Name: "computer.key_repeat_max", Kind: config.KindInt, Default: "50", Min: "1", Chapter: chapter27,
			Doc: "Highest repeat of one key action of a computer session."},
		{Name: "computer.audit_apps_max", Kind: config.KindInt, Default: "32", Min: "0", Chapter: chapter27,
			Doc: "Most app identities in the audit entry of the end of a computer session."},
		{Name: "computer.status_interval", Kind: config.KindDuration, Default: "2s", Min: "1s", Chapter: chapter27,
			Doc: "Shortest interval between two computer.state reports of a running computer session."},
	}
}
