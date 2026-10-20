// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

func TestParseRange_Table(t *testing.T) {
	const etag = `"5d41402abc4b2a76b9719d911017c592"`
	full := domain.RangeResult{Kind: domain.RangeFull}
	unsat := domain.RangeResult{Kind: domain.RangeUnsatisfiable}
	part := func(first, last int64) domain.RangeResult {
		return domain.RangeResult{Kind: domain.RangePartial, First: first, Last: last}
	}
	for _, c := range []struct {
		name, header, ifRange, etag string
		size                        int64
		want                        domain.RangeResult
	}{
		{"no header", "", "", etag, 1000, full},
		{"first-last", "bytes=0-99", "", etag, 1000, part(0, 99)},
		{"first-", "bytes=100-", "", etag, 1000, part(100, 999)},
		{"-suffix", "bytes=-10", "", etag, 1000, part(990, 999)},
		{"single byte", "bytes=5-5", "", etag, 1000, part(5, 5)},
		{"last byte", "bytes=999-999", "", etag, 1000, part(999, 999)},
		{"last capped", "bytes=900-5000", "", etag, 1000, part(900, 999)},
		{"suffix longer than size", "bytes=-5000", "", etag, 1000, part(0, 999)},
		{"suffix equal to size", "bytes=-1000", "", etag, 1000, part(0, 999)},
		{"unit case-insensitive", "Bytes=0-0", "", etag, 1000, part(0, 0)},
		{"unit upper case", "BYTES=1-2", "", etag, 1000, part(1, 2)},
		{"whitespace around spec", "bytes= \t1-2 \t", "", etag, 1000, part(1, 2)},
		{"leading zeros", "bytes=007-010", "", etag, 1000, part(7, 10)},
		{"other unit", "items=0-1", "", etag, 1000, full},
		{"no unit", "=0-1", "", etag, 1000, full},
		{"no equals sign", "bytes 0-1", "", etag, 1000, full},
		{"space before equals", "bytes =0-1", "", etag, 1000, full},
		{"two ranges", "bytes=0-1,5-6", "", etag, 1000, full},
		{"two ranges with space", "bytes=0-1, 5-6", "", etag, 1000, full},
		{"trailing comma", "bytes=0-1,", "", etag, 1000, full},
		{"two unsatisfiable ranges", "bytes=2000-2001,3000-", "", etag, 1000, full},
		{"empty spec", "bytes=", "", etag, 1000, full},
		{"dash only", "bytes=-", "", etag, 1000, full},
		{"last before first", "bytes=10-5", "", etag, 1000, full},
		{"non-digits", "bytes=a-b", "", etag, 1000, full},
		{"hex", "bytes=0x1-2", "", etag, 1000, full},
		{"plus sign", "bytes=+1-2", "", etag, 1000, full},
		{"minus sign in last", "bytes=1--2", "", etag, 1000, full},
		{"negative suffix", "bytes=--2", "", etag, 1000, full},
		{"space inside number", "bytes=1 0-20", "", etag, 1000, full},
		{"space around dash", "bytes=10 - 20", "", etag, 1000, full},
		{"underscore", "bytes=1_0-20", "", etag, 1000, full},
		{"non-ASCII digit", "bytes=١-2", "", etag, 1000, full},
		{"overflow first", "bytes=" + tooBig + "-", "", etag, 1000, full},
		{"overflow last", "bytes=0-" + tooBig, "", etag, 1000, full},
		{"overflow suffix", "bytes=-" + tooBig, "", etag, 1000, full},
		{"max int64 last", "bytes=0-" + strconv.FormatInt(math.MaxInt64, 10), "", etag, 1000, part(0, 999)},
		{"start at size", "bytes=1000-", "", etag, 1000, unsat},
		{"start at size with last", "bytes=1000-1001", "", etag, 1000, unsat},
		{"start beyond size", "bytes=5000-6000", "", etag, 1000, unsat},
		{"zero suffix", "bytes=-0", "", etag, 1000, unsat},
		{"empty variant", "bytes=0-", "", etag, 0, unsat},
		{"empty variant suffix", "bytes=-5", "", etag, 0, unsat},
		{"If-Range equal", "bytes=0-99", etag, etag, 1000, part(0, 99)},
		{"If-Range equal unsatisfiable", "bytes=1000-", etag, etag, 1000, unsat},
		{"If-Range different", "bytes=0-99", `"other"`, etag, 1000, full},
		{"If-Range different unsatisfiable", "bytes=1000-", `"other"`, etag, 1000, full},
		{"If-Range date", "bytes=0-99", "Tue, 20 Oct 2026 12:00:00 GMT", etag, 1000, full},
		{"If-Range weak", "bytes=0-99", "W/" + etag, "W/" + etag, 1000, full},
		{"If-Range weak against strong", "bytes=0-99", "W/" + etag, etag, 1000, full},
		{"If-Range unquoted", "bytes=0-99", "abc", "abc", 1000, full},
		{"If-Range case differs", "bytes=0-99", `"ABC"`, `"abc"`, 1000, full},
		{"If-Range without Range", "", etag, etag, 1000, full},
		{"If-Range with empty ETag", "bytes=0-99", `""`, "", 1000, full},
	} {
		if got := domain.ParseRange(c.header, c.ifRange, c.etag, c.size); got != c.want {
			t.Errorf("%s: ParseRange(%q, %q, size %d) = %+v, want %+v", c.name, c.header, c.ifRange, c.size, got, c.want)
		}
	}
}

// tooBig is the largest int64 plus one in decimal, built so no long digit literal is in the
// source.
var tooBig = strconv.FormatUint(uint64(math.MaxInt64)+1, 10)

func FuzzParseRange(f *testing.F) {
	for _, s := range []string{"", "bytes=0-99", "bytes=100-", "bytes=-10", "bytes=-0", "bytes=0-1,2-3", "bytes=" + tooBig + "-", "BYTES= 5-4 "} {
		f.Add(s, "", int64(1000))
		f.Add(s, `"e"`, int64(0))
		f.Add(s, `"e"`, int64(-1))
	}
	f.Fuzz(func(t *testing.T, header, ifRange string, size int64) {
		got := domain.ParseRange(header, ifRange, `"e"`, size)
		switch got.Kind {
		case domain.RangePartial:
			if got.First < 0 || got.First > got.Last || got.Last >= size {
				t.Fatalf("partial range %d-%d outside a variant of %d bytes", got.First, got.Last, size)
			}
		case domain.RangeFull, domain.RangeUnsatisfiable:
			if got.First != 0 || got.Last != 0 {
				t.Fatalf("kind %d carries a range", got.Kind)
			}
		default:
			t.Fatalf("unknown kind %d", got.Kind)
		}
		if header == "" && got.Kind != domain.RangeFull {
			t.Fatal("no Range header but not Full")
		}
		if ifRange != "" && ifRange != `"e"` && got.Kind != domain.RangeFull {
			t.Fatal("a different If-Range did not give Full")
		}
	})
}
