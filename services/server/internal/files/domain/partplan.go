// SPDX-License-Identifier: Apache-2.0

package domain

// PlanParts returns the part plan of an upload of size bytes in parts of partSize bytes
// (docs/spec/12-files.md "3.1"): count is size / partSize rounded up, last is the length of the
// last part, which is the rest and may be smaller. Both arguments are at least 1 (the schema
// guarantees size_bytes >= 1 and the configuration file.upload_part_bytes >= 1); for other
// values it returns 0, 0.
//
// Example:
//
//	count, last := domain.PlanParts(25, 8) // 4, 1
func PlanParts(size, partSize int64) (count int, last int64) {
	if size < 1 || partSize < 1 {
		return 0, 0
	}
	n := (size + partSize - 1) / partSize
	return int(n), size - (n-1)*partSize
}

// PartLength returns the length of part n (from 1) of a plan of count parts for size bytes in
// parts of partSize bytes: partSize for every part but the last, the rest for the last, and 0 for
// a number outside 1 to count.
//
// Example:
//
//	domain.PartLength(4, 4, 25, 8) // 1
func PartLength(n, count int, size, partSize int64) int64 {
	if n < 1 || n > count {
		return 0
	}
	if n < count {
		return partSize
	}
	return size - int64(count-1)*partSize
}
