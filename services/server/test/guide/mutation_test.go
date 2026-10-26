// SPDX-License-Identifier: Apache-2.0

package guide

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// mustParse parses a page text with the step titles of the repository and validates it.
func mustParse(t *testing.T, root, text string) *Page {
	t.Helper()
	page, err := staticCheck(root, text)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

// staticCheck runs every check that comes before a server start.
func staticCheck(root, text string) (*Page, error) {
	chapter, err := os.ReadFile(filepath.Join(root, ChapterPath)) // #nosec G304 -- a fixed file of the repository
	if err != nil {
		return nil, err
	}
	titles, err := StepTitles(chapter)
	if err != nil {
		return nil, err
	}
	page, err := Parse([]byte(text), titles)
	if err != nil {
		return nil, err
	}
	in, err := LoadInputs(root)
	if err != nil {
		return nil, err
	}
	return page, Validate(page, in)
}

// section returns the text of a step section, from the line break before its heading to the
// line break before the next heading.
func section(t *testing.T, text string, step string) string {
	t.Helper()
	start := strings.Index(text, "\n## Step "+step+": ")
	if start < 0 {
		t.Fatalf("no step %s on the page", step)
	}
	end := strings.Index(text[start+1:], "\n## ")
	if end < 0 {
		return text[start:]
	}
	return text[start : start+1+end]
}

// replaceOnce replaces old with new once and fails when old is not on the page.
func replaceOnce(t *testing.T, text, old, new string) string {
	t.Helper()
	if !strings.Contains(text, old) {
		t.Fatalf("the page does not hold %q", old)
	}
	return strings.Replace(text, old, new, 1)
}

func TestMutations_Static(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, PagePath)) // #nosec G304 -- a fixed file of the repository
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if _, err := staticCheck(root, text); err != nil {
		t.Fatalf("the real page fails the static checks: %v", err)
	}
	step5, step6 := section(t, text, "5"), section(t, text, "6")
	pin := regexp.MustCompile(`"bothy-sdk==[^"]+"`)
	cases := []struct {
		name   string
		mutate func(*testing.T, string) string
		want   []string
	}{
		{"changed line in a shown file", func(t *testing.T, s string) string {
			return replaceOnce(t, s, `ADAPTER_VERSION = "1.0.0"`, `ADAPTER_VERSION = "1.0.1"`)
		}, []string{"Step 4", "connect.py", "differs"}},
		{"steps 5 and 6 swapped", func(t *testing.T, s string) string {
			s = replaceOnce(t, s, step5, "\x00")
			s = replaceOnce(t, s, step6, step5)
			return replaceOnce(t, s, "\x00", step6)
		}, []string{"Step 5"}},
		{"step 9 removed", func(t *testing.T, s string) string {
			return replaceOnce(t, s, section(t, s, "9"), "\n")
		}, []string{"Step 9"}},
		{"unknown operationId", func(t *testing.T, s string) string {
			return replaceOnce(t, s, `title="Owner: getAgent"`, `title="Owner: readAgent"`)
		}, []string{"Step 5", "unknown operationId readAgent"}},
		{"wrong path", func(t *testing.T, s string) string {
			return replaceOnce(t, s, "GET /api/v1/agents/{agent_id} HTTP/1.1", "GET /api/v1/agent/{agent_id} HTTP/1.1")
		}, []string{"Step 5", "does not fit"}},
		{"unbound placeholder", func(t *testing.T, s string) string {
			return replaceOnce(t, s, "GET /api/v1/agents/{agent_id} HTTP/1.1", "GET /api/v1/agents/{agent_uuid} HTTP/1.1")
		}, []string{"Step 5", "{agent_uuid}"}},
		{"untitled block", func(t *testing.T, s string) string {
			return replaceOnce(t, s, "```http title=\"Owner: getAgent\"", "```http")
		}, []string{"Step 5", "line ", "without a language and a title"}},
		{"changed pin", func(t *testing.T, s string) string {
			return pin.ReplaceAllString(s, `"bothy-sdk==9.9.9"`)
		}, []string{"Step 10", "bothy-sdk==9.9.9"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := staticCheck(root, c.mutate(t, text))
			if err == nil {
				t.Fatal("the mutated page passed the static checks")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("err = %v, want one with %q", err, w)
				}
			}
		})
	}
}
