// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"fmt"
	"strconv"
)

// Seed returns the seed of a G7 run from the value of CONTRACT_SEED, a decimal unsigned integer.
// make contract-test always sets it (named value quality.contract.seed, docs/spec/23-quality.md
// 11), so every run draws the same cases and a failure repeats; G7 never draws a seed of its own.
func Seed(value string) (string, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return "", fmt.Errorf("CONTRACT_SEED %q is no decimal unsigned integer (make contract-test sets quality.contract.seed): %w", value, err)
	}
	return strconv.FormatUint(n, 10), nil
}
