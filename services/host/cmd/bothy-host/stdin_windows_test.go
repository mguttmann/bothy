// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// keepStdin is not needed on Windows: the helper creates its own named pipe.
func keepStdin(*testing.T) {}
