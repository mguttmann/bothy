// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"strconv"
	"strings"
)

// MissingParts returns the part numbers from 1 to count that are not in confirmed, ascending.
// Numbers outside 1 to count in confirmed are ignored.
//
// Example:
//
//	domain.MissingParts(9, []int{1, 2, 3, 7, 8}) // [4 5 6 9]
func MissingParts(count int, confirmed []int) []int {
	have := make(map[int]bool, len(confirmed))
	for _, n := range confirmed {
		have[n] = true
	}
	var out []int
	for n := 1; n <= count; n++ {
		if !have[n] {
			out = append(out, n)
		}
	}
	return out
}

// Ranges writes ascending part numbers as comma separated ranges, the detail of
// 409 upload_incomplete (docs/spec/12-files.md "3.4" point 1): consecutive numbers become
// "a-b", single numbers stay. An empty list gives "".
//
// Example:
//
//	domain.Ranges([]int{4, 5, 6, 9}) // "4-6,9"
func Ranges(numbers []int) string {
	numbers = slices.Clone(numbers)
	slices.Sort(numbers)
	var b strings.Builder
	for i := 0; i < len(numbers); {
		j := i
		for j+1 < len(numbers) && numbers[j+1] == numbers[j]+1 {
			j++
		}
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(numbers[i]))
		if j > i {
			b.WriteByte('-')
			b.WriteString(strconv.Itoa(numbers[j]))
		}
		i = j + 1
	}
	return b.String()
}
