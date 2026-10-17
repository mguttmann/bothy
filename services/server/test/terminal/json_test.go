// SPDX-License-Identifier: Apache-2.0

package terminal

import "encoding/json"

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
