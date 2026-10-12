// SPDX-License-Identifier: Apache-2.0

package jsoncanon_test

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/jsoncanon"
)

// TestJCS_RFC8785Vectors holds the examples of RFC 8785: section 3.2.2 (values), section
// 3.2.3 (sorting by UTF-16 code units) and numbers of Appendix B.
func TestJCS_RFC8785Vectors(t *testing.T) {
	cases := map[string]struct{ in, want string }{
		"3.2.2 values": {
			in: `{
  "numbers": [333333333.33333329, 1E30, 4.50, 2e-3, 0.000000000000000000000000001],
  "string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/",
  "literals": [null, true, false]
}`,
			want: `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`,
		},
		"3.2.3 sorting": {
			in: `{
  "\u20ac": "Euro Sign",
  "\r": "Carriage Return",
  "\ufb33": "Hebrew Letter Dalet With Dagesh",
  "1": "One",
  "\ud83d\ude00": "Emoji: Grinning Face",
  "\u0080": "Control",
  "\u00f6": "Latin Small Letter O With Diaeresis"
}`,
			want: "{\"\\r\":\"Carriage Return\",\"1\":\"One\",\"\u0080\":\"Control\",\"\u00f6\":\"Latin Small Letter O With Diaeresis\",\"\u20ac\":\"Euro Sign\",\"\U0001F600\":\"Emoji: Grinning Face\",\"\ufb33\":\"Hebrew Letter Dalet With Dagesh\"}",
		},
		"appendix B zero":     {in: `[-0, 0.0]`, want: `[0,0]`},
		"appendix B 1e21":     {in: `1e21`, want: `1e+21`},
		"appendix B max safe": {in: `9007199254740991`, want: `9007199254740991`},
		"appendix B 1e-7":     {in: `0.0000001`, want: `1e-7`},
		"appendix B 1e23":     {in: `1e23`, want: `1e+23`},
		"appendix B small":    {in: `5e-324`, want: `5e-324`},
		"whitespace":          {in: " { \"b\" : [ 1 , 2 ] , \"a\" : { } } ", want: `{"a":{},"b":[1,2]}`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := jsoncanon.Canonicalize([]byte(c.in))
			if err != nil {
				t.Fatalf("Canonicalize: %v", err)
			}
			if string(got) != c.want {
				t.Errorf("Canonicalize\n got %s\nwant %s", got, c.want)
			}
		})
	}
}

// TestCanonicalize_Refuses lists the inputs without a canonical form: not JSON, a duplicate
// member at any level, and numbers that IEEE 754 double precision cannot hold exactly, which
// RFC 8785 would round and so give two different requests the same form.
func TestCanonicalize_Refuses(t *testing.T) {
	for name, in := range map[string]string{
		"not json":         `{"a":`,
		"trailing value":   `{} {}`,
		"duplicate":        `{"a":1,"a":2}`,
		"nested duplicate": `{"x":[{"a":1,"a":1}]}`,
		"big integer":      `{"n":` + strconv.FormatUint(math.MaxUint64, 10) + `}`,
		"many digits":      "1" + strings.Repeat("0", 400),
		"above max safe":   `9007199254740993`,
		"overflow":         `1e400`,
		"invalid utf-8":    "\"\xff\"",
		"lone surrogate":   `"\ud800"`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := jsoncanon.Canonicalize([]byte(in)); !errors.Is(err, jsoncanon.ErrNoCanonicalForm) {
				t.Errorf("Canonicalize(%q) error = %v, want ErrNoCanonicalForm", in, err)
			}
		})
	}
}
