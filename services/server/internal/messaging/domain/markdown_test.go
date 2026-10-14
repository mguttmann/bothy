// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// codeRuns returns the code parts of content as the scanner sees them: the maximal runs of bytes
// inside code, without the runs that hold only white space (a blank line inside or after a code
// block is irrelevant for mentions).
func codeRuns(content string) []string {
	mask := domain.CodeMask(content)
	var out []string
	for i := 0; i < len(content); {
		if !mask[i] {
			i++
			continue
		}
		j := i
		for j < len(content) && mask[j] {
			j++
		}
		if run := content[i:j]; strings.TrimSpace(run) != "" {
			out = append(out, run)
		}
		i = j
	}
	return out
}

// commonMarkCases are the examples of the CommonMark specification 0.31.2 for indented code
// blocks (107 to 118), fenced code blocks (119 to 147) and code spans (328 to 349), with the
// code the scanner must skip (brief M1-56 D5). A code block covers its lines including the
// fences and the container markers in front of them; a code span covers its backticks.
var commonMarkCases = []struct {
	example int
	in      string
	code    []string
}{
	{107, "    a simple\n      indented code block\n", []string{"    a simple", "      indented code block"}},
	{108, "  - foo\n\n    bar\n", nil},
	{109, "1.  foo\n\n    - bar\n", nil},
	{110, "    <a/>\n    *hi*\n\n    - one\n", []string{"    <a/>", "    *hi*", "    - one"}},
	{111, "    chunk1\n\n    chunk2\n  \n \n \n    chunk3\n", []string{"    chunk1", "    chunk2", "    chunk3"}},
	{112, "    chunk1\n      \n      chunk2\n", []string{"    chunk1", "      chunk2"}},
	{113, "Foo\n    bar\n", nil},
	{114, "    foo\nbar\n", []string{"    foo"}},
	{115, "# Heading\n    foo\nHeading\n------\n    foo\n----\n", []string{"    foo", "    foo"}},
	{116, "        foo\n    bar\n", []string{"        foo", "    bar"}},
	{117, "\n    \n    foo\n    \n\n", []string{"    foo"}},
	{118, "    foo  \n", []string{"    foo  "}},
	{119, "```\n<\n >\n```\n", []string{"```", "<", " >", "```"}},
	{120, "~~~\n<\n >\n~~~\n", []string{"~~~", "<", " >", "~~~"}},
	{121, "``\nfoo\n``\n", []string{"``\nfoo\n``"}},
	{122, "```\naaa\n~~~\n```\n", []string{"```", "aaa", "~~~", "```"}},
	{123, "~~~\naaa\n```\n~~~\n", []string{"~~~", "aaa", "```", "~~~"}},
	{124, "````\naaa\n```\n``````\n", []string{"````", "aaa", "```", "``````"}},
	{125, "~~~~\naaa\n~~~\n~~~~\n", []string{"~~~~", "aaa", "~~~", "~~~~"}},
	{126, "```\n", []string{"```"}},
	{127, "`````\n\n```\naaa\n", []string{"`````", "```", "aaa"}},
	{128, "> ```\n> aaa\n\nbbb\n", []string{"> ```", "> aaa"}},
	{129, "```\n\n  \n```\n", []string{"```", "```"}},
	{130, "```\n```\n", []string{"```", "```"}},
	{131, " ```\n aaa\naaa\n```\n", []string{" ```", " aaa", "aaa", "```"}},
	{132, "  ```\naaa\n  aaa\naaa\n  ```\n", []string{"  ```", "aaa", "  aaa", "aaa", "  ```"}},
	{133, "   ```\n   aaa\n    aaa\n  aaa\n   ```\n", []string{"   ```", "   aaa", "    aaa", "  aaa", "   ```"}},
	{134, "    ```\n    aaa\n    ```\n", []string{"    ```", "    aaa", "    ```"}},
	{135, "```\naaa\n  ```\n", []string{"```", "aaa", "  ```"}},
	{136, "   ```\naaa\n  ```\n", []string{"   ```", "aaa", "  ```"}},
	{137, "```\naaa\n    ```\n", []string{"```", "aaa", "    ```"}},
	{138, "``` ```\naaa\n", []string{"``` ```"}},
	{139, "~~~~~~\naaa\n~~~ ~~\n", []string{"~~~~~~", "aaa", "~~~ ~~"}},
	{140, "foo\n```\nbar\n```\nbaz\n", []string{"```", "bar", "```"}},
	{141, "foo\n---\n~~~\nbar\n~~~\n# baz\n", []string{"~~~", "bar", "~~~"}},
	{142, "```ruby\ndef foo(x)\n  return 3\nend\n```\n", []string{"```ruby", "def foo(x)", "  return 3", "end", "```"}},
	{143, "~~~~    ruby startline=3 $%@#$\ndef foo(x)\n  return 3\nend\n~~~~~~~\n",
		[]string{"~~~~    ruby startline=3 $%@#$", "def foo(x)", "  return 3", "end", "~~~~~~~"}},
	{144, "````;\n````\n", []string{"````;", "````"}},
	{145, "``` aa ```\nfoo\n", []string{"``` aa ```"}},
	{146, "~~~ aa ``` ~~~\nfoo\n~~~\n", []string{"~~~ aa ``` ~~~", "foo", "~~~"}},
	{147, "```\n``` aaa\n```\n", []string{"```", "``` aaa", "```"}},
	{328, "`foo`", []string{"`foo`"}},
	{329, "`` foo ` bar ``", []string{"`` foo ` bar ``"}},
	{330, "` `` `", []string{"` `` `"}},
	{331, "`  ``  `", []string{"`  ``  `"}},
	{332, "` a`", []string{"` a`"}},
	{333, "`\u00a0b\u00a0`", []string{"`\u00a0b\u00a0`"}},
	{334, "` `\n`  `", []string{"` `", "`  `"}},
	{335, "``\nfoo\nbar  \nbaz\n``", []string{"``\nfoo\nbar  \nbaz\n``"}},
	{336, "``\nfoo \n``", []string{"``\nfoo \n``"}},
	{337, "`foo   bar \nbaz`", []string{"`foo   bar \nbaz`"}},
	{338, "`foo\\`bar`", []string{"`foo\\`"}},
	{339, "``foo`bar``", []string{"``foo`bar``"}},
	{340, "` foo `` bar `", []string{"` foo `` bar `"}},
	{341, "*foo`*`", []string{"`*`"}},
	{342, "[not a `link](/foo`)", []string{"`link](/foo`"}},
	{343, "`<a href=\"`\">`", []string{"`<a href=\"`"}},
	// Example 344 is raw HTML in CommonMark. Content format 1 has no raw HTML (03 messages,
	// SEC-OUT-1), so the tag is text and its backticks open a code span (docs/DECISIONS.md M1-56).
	{344, "<a href=\"`\">`", []string{"`\">`"}},
	{345, "`<https://foo.bar.`baz>`", []string{"`<https://foo.bar.`"}},
	{346, "<https://foo.bar.`baz>`", nil},
	{347, "```foo``", nil},
	{348, "`foo", nil},
	{349, "`foo``bar``", []string{"``bar``"}},
}

// TestCodeMask_CommonMarkExamples: the scanner finds the code of every code-span and code-block
// example of CommonMark 0.31.2 (brief M1-56 D5).
func TestCodeMask_CommonMarkExamples(t *testing.T) {
	t.Parallel()
	for _, c := range commonMarkCases {
		if got := codeRuns(c.in); !slices.Equal(got, c.code) {
			t.Errorf("example %d %q: code %q, want %q", c.example, c.in, got, c.code)
		}
	}
}

// TestCodeMask_Containers: code blocks inside list items and block quotes, tabs, lazy
// continuation and an e-mail autolink with a backtick.
func TestCodeMask_Containers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		code []string
	}{
		{"- foo\n\n      bar\n", []string{"      bar"}},
		{"- ```\n  aaa\n- bbb\n", []string{"- ```", "  aaa"}},
		{"> foo\n>\n>     code\n", []string{">     code"}},
		{"> foo\nbar `x`\n", []string{"`x`"}},
		{"\tcode\n", []string{"\tcode"}},
		{"-\tfoo\n\n\t  bar\n", nil},
		{"<a`b@example.com> `c`", []string{"`c`"}},
		{"\\`not code`", nil},
		{"1. a\n2) b\n\n        code\n", []string{"        code"}},
	}
	for _, c := range cases {
		if got := codeRuns(c.in); !slices.Equal(got, c.code) {
			t.Errorf("%q: code %q, want %q", c.in, got, c.code)
		}
	}
}

// TestCodeMask_DeepListsLinear: inputs of message.max_content_bytes with thousands of nested
// containers, blank or lazy lines, backtick runs and autolink look-alikes are scanned in time
// linear in their size, so one message cannot occupy a core (code review M1-56). Each input is
// compared with a flat list of the same size scanned just before it, so the bound follows the
// load of the machine (race detector, coverage, parallel suites); the quadratic scanner took
// hundreds of times as long as that baseline.
func TestCodeMask_DeepListsLinear(t *testing.T) {
	t.Parallel()
	var runs strings.Builder
	for i := 1; runs.Len() < 260000; i++ {
		runs.WriteString(strings.Repeat("`", i%700+1) + "a")
	}
	inputs := map[string]string{
		"lists and blank lines":  strings.Repeat("- ", 65000) + "x" + strings.Repeat("\n", 130000),
		"quotes and blank lines": strings.Repeat("> ", 65000) + "x" + strings.Repeat("\n", 130000),
		"lists and lazy lines":   strings.Repeat("- ", 65000) + "x" + strings.Repeat("\ny", 65000),
		"mixed containers":       strings.Repeat("- > 1. ", 37000) + "x" + strings.Repeat("\n", 1000),
		"backtick runs":          runs.String(),
		"autolink look-alikes":   strings.Repeat("<a", 130000),
		"indented continuation":  strings.Repeat("- ", 43000) + "x\n" + strings.Repeat(" ", 86000) + "y",
	}
	flat := strings.Repeat("- x\n", 65000) + " @a"
	for name, in := range inputs {
		began := time.Now()
		domain.ScanMentions(flat)
		base := time.Since(began)
		began = time.Now()
		if got := domain.ScanMentions(in + " @a"); !slices.Equal(got, []string{"a"}) {
			t.Fatalf("%s: mentions %v", name, got)
		}
		if d := time.Since(began); d > 10*base+200*time.Millisecond {
			t.Errorf("%s: scan took %v, a flat list of the same size %v", name, d, base)
		}
	}
}

// TestScanMentions_BlankAfterQuoteMarkers (CommonMark 0.31.2 example 262 and red team M1-56): a
// line that is blank after its block quote markers continues the list items inside the quote, so
// a fenced code block in such an item stays open and a paragraph after it stays in the item.
func TestScanMentions_BlankAfterQuoteMarkers(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"> - ```\n>   x\n>\n>   @a\n>   ```":      nil,
		"   > > 1.  one\n>>\n>>     @two":         {"two"},
		"> - a\n>\n>       @code\n":               nil,
		"> -\n>\n>   @after\n":                    {"after"},
		"> - ```\n>   x\n>  \t \n>   @b\n>   ```": nil,
	}
	for in, want := range cases {
		if got := domain.ScanMentions(in); !slices.Equal(got, want) {
			t.Errorf("%q: mentions %v, want %v", in, got, want)
		}
	}
}
